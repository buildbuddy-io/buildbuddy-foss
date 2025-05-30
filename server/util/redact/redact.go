package redact

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/util/flag"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/google/shlex"
	"google.golang.org/protobuf/encoding/prototext"

	bespb "github.com/buildbuddy-io/buildbuddy/proto/build_event_stream"
	clpb "github.com/buildbuddy-io/buildbuddy/proto/command_line"
	gitutil "github.com/buildbuddy-io/buildbuddy/server/util/git"
)

var (
	apiKey = flag.String(
		"api.api_key",
		"",
		"The default API key to use for on-prem enterprise deploys with a single organization/group.",
		flag.Secret,
		flag.Deprecated(
			"Manual API key specification is no longer supported; to retrieve specific API keys programmatically, please use the API key table. This field will still specify an API key to redact in case a manual API key was specified when buildbuddy was first set up.",
		),
	)
)

const (
	RedactionFlagStandardRedactions = 1

	envVarPrefix        = "--"
	envVarSeparator     = "="
	redactedPlaceholder = "<REDACTED>"

	buildMetadataOptionPrefix = "--build_metadata="
	allowEnvPrefix            = "ALLOW_ENV="
	allowEnvListSeparator     = ","
	explicitCommandLineName   = "EXPLICIT_COMMAND_LINE"
)

var (
	envVarOptionNames      = []string{"action_env", "client_env", "host_action_env", "repo_env", "test_env"}
	envVarOptionNamesRegex = regexp.MustCompile(`(--(?:` + strings.Join(envVarOptionNames, "|") + `)=\w+=).*`)

	urlSecretRegex      = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^:@]+:)[^@]*(@[^"\s<>{}|\\^[\]]+)`)
	residualSecretRegex = regexp.MustCompile(`(?i)` + `(^|[^a-z])` + `(api|key|pass|password|secret|token)` + `([^a-z]|$)`)

	// There are some flags that contain multiple sub-flags which are
	// specified as comma-separated KEY=VALUE pairs, e.g.:
	//     --build_metadata=ONE=foo,TWO=bar,THREE=baz
	// These can cause problems with the flag value redaction logic if the
	// first character of the value is an '@' because 'NAME=@VALUE' matches the
	// above regex. We could add some very fancy (complicated) flag parsing and
	// redaction logic, but for now we'll just permit certain flags to be
	// parsed this way and have only the sub-values undergo redaction. This is
	// the list of flag names that are permitted to be treated this way.
	knownMultiFlags   = []string{"build_metadata"}
	multiFlagKeyRegex = regexp.MustCompile(`,[A-Z_]+=`)

	knownGitRepoURLKeys = []string{
		"REPO_URL", "GIT_URL", "TRAVIS_REPO_SLUG", "BUILDKITE_REPO", "GIT_REPOSITORY_URL",
		"CIRCLE_REPOSITORY_URL", "GITHUB_REPOSITORY", "CI_REPOSITORY_URL",
	}

	defaultAllowedEnvVars = []string{
		"USER", "GITHUB_ACTOR", "BUILDKITE_BUILD_CREATOR", "GITLAB_USER_NAME", "CIRCLE_USERNAME", "GITHUB_SHA", "GITHUB_RUN_ID",
		"BUILDKITE_BUILD_URL", "BUILDKITE_JOB_ID", "CIRCLE_REPOSITORY_URL",
		"GITHUB_REPOSITORY", "BUILDKITE_REPO", "TRAVIS_REPO_SLUG", "GIT_URL", "GIT_REPOSITORY_URL",
		"CI_REPOSITORY_URL", "REPO_URL", "CIRCLE_SHA1", "BUILDKITE_COMMIT",
		"TRAVIS_COMMIT", "BITRISE_GIT_COMMIT", "GIT_COMMIT", "VOLATILE_GIT_COMMIT", "CI_COMMIT_SHA",
		"COMMIT_SHA", "CI", "CI_RUNNER", "CIRCLE_BRANCH", "GITHUB_HEAD_REF", "BUILDKITE_BRANCH", "TRAVIS_BRANCH",
		"BITRISE_GIT_BRANCH", "GIT_BRANCH", "CI_COMMIT_BRANCH", "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "GITHUB_REF",
	}

	redactedPlatformProps = []string{
		"container-registry-username", "container-registry-password",
	}

	// Here we match 20 alphanumeric characters preceded by the api key header flag
	apiKeyHeaderPattern = regexp.MustCompile("x-buildbuddy-api-key=[[:alnum:]]{20}")

	// Match sequences that look like API keys immediately followed '@',
	// to account for patterns like "grpc://$API_KEY@app.buildbuddy.io"
	// or "bes_backend=$API_KEY@domain.com".
	apiKeyAtPattern = regexp.MustCompile("(^|[/=])[[:alnum:]]{20}@")

	// Option names which may contain gRPC headers that should be redacted.
	headerOptionNames = []string{
		"remote_header",
		"remote_cache_header",
		"remote_exec_header",
		"remote_downloader_header",
		"bes_header",
	}
)

func stripURLSecrets(input string) string {
	return urlSecretRegex.ReplaceAllString(input, "${1}<REDACTED>${2}")
}

// Strips URL secrets from the provided flag value, if there is a value.
func stripUrlSecretsFromFlag(input string) string {
	ck, cv := splitCombinedForm(input)
	if cv == "" {
		// No flag value to redact; keep flag name as-is.
		return input
	}
	return ck + "=" + stripURLSecrets(cv)
}

// Strips URL secrets from all of the provided multi-flags. These are flags
// that are specified as:
//
//	--flag_name=ONE=foo,TWO=BAR,THREE=baz
//
// This function will strip secrets from the "foo", "bar", and "baz" components,
// leaving "--flag_name", "ONE", "TWO", and "THREE" intact.
func stripUrlSecretsFromMultiFlag(input string) string {
	key, val := splitCombinedForm(input)
	return key + "=" + stripUrlSecretsFromMultiFlagValue(val)
}

// Same as above but expects to not get the "--flag_name=" part.
func stripUrlSecretsFromMultiFlagValue(input string) string {
	subFlags := splitMultiFlag(input)
	for i, flag := range subFlags {
		subFlags[i] = stripUrlSecretsFromFlag(flag)
	}
	return strings.Join(subFlags, ",")
}

// Splits the provided multi-flags into an array of regular ole' flags. So,
// "ONE=foo,TWO=bar,THREE=baz" becomes ["ONE=foo", "TWO=bar", "THREE=baz"]
func splitMultiFlag(input string) []string {
	// multiFlagKeyRegex only matches the flag name for the 2nd and beyond flags
	// so prepend a 0 to capture the first sub-flag.
	subFlags := multiFlagKeyRegex.FindAllStringIndex(input, -1 /* return all matches */)
	subFlagStarts := make([]int, len(subFlags)+1)
	subFlagStarts[0] = 0
	for i := 0; i < len(subFlags); i++ {
		subFlagStarts[i+1] = subFlags[i][0]
	}

	output := make([]string, len(subFlagStarts))
	for i := 0; i < len(subFlagStarts); i++ {
		start := subFlagStarts[i]
		if start > 0 {
			// Skip the leading comma
			start++
		}
		end := len(input)
		if i+1 < len(subFlagStarts) {
			end = subFlagStarts[i+1]
		}
		output[i] = input[start:end]
	}
	return output
}

func isKnownMultiFlag(input string) bool {
	for _, knownMultiFlag := range knownMultiFlags {
		if strings.HasPrefix(input, knownMultiFlag) || strings.HasPrefix(input, "--"+knownMultiFlag) {
			return true
		}
	}
	return false
}

func stripURLSecretsFromCmdLine(tokens []string) {
	for i, token := range tokens {
		if isKnownMultiFlag(token) {
			tokens[i] = stripUrlSecretsFromMultiFlag(token)
		} else if strings.HasPrefix(token, "--") {
			tokens[i] = stripUrlSecretsFromFlag(token)
		} else {
			tokens[i] = stripURLSecrets(token)
		}
	}
}

func stripRemoteHeadersFromCmdLine(tokens []string) {
	for i, token := range tokens {
		for _, name := range headerOptionNames {
			if strings.HasPrefix(token, "--"+name+"=") {
				tokens[i] = "--" + name + "=<REDACTED>"
				break
			}
		}
	}
}

func stripExplicitCommandLineFromCmdLine(tokens []string) {
	for i, token := range tokens {
		if strings.HasPrefix(token, buildMetadataOptionPrefix+explicitCommandLineName+"=") {
			tokens[i] = ""
		}
	}
}

func stripNonAllowedEnvVars(tokens []string) {
	for i, token := range tokens {
		tokens[i] = envVarOptionNamesRegex.ReplaceAllString(token, "${1}<REDACTED>")
	}
}

func redactCmdLine(tokens []string) {
	stripURLSecretsFromCmdLine(tokens)
	stripRemoteHeadersFromCmdLine(tokens)
	stripExplicitCommandLineFromCmdLine(tokens)
	stripNonAllowedEnvVars(tokens)
}

func RedactText(txt string) string {
	txt = stripURLSecrets(txt)
	txt = redactRemoteHeaders(txt)
	txt = redactBuildBuddyAPIKeys(txt)
	return txt
}

// redactBuildBuddyAPIKeys redacts BuildBuddy API keys in the input string.
// It looks for HTTP headers, URL secrets, environment variables, and the configured API key.
// This implementation depends on BuildBuddy API keys being exactly 20 alphanumeric characters.
func redactBuildBuddyAPIKeys(txt string) string {
	// Replace x-buildbuddy-api-key header.
	txt = apiKeyHeaderPattern.ReplaceAllLiteralString(txt, "x-buildbuddy-api-key=<REDACTED>")

	// Replace sequences that look like API keys immediately followed by '@',
	// to account for patterns like "grpc://$API_KEY@app.buildbuddy.io"
	// or "bes_backend=$API_KEY@domain.com".
	txt = apiKeyAtPattern.ReplaceAllString(txt, "$1<REDACTED>@")

	// Replace the literal API key set up via the BuildBuddy config, which does not
	// need to conform to the way we generate API keys.
	if configuredKey := *apiKey; configuredKey != "" {
		txt = strings.ReplaceAll(txt, configuredKey, "<REDACTED>")
	}

	return txt
}

func redactRemoteHeaders(txt string) string {
	for _, header := range headerOptionNames {
		regex := regexp.MustCompile(fmt.Sprintf("--%s=[^\\s]+", header))
		txt = regex.ReplaceAllLiteralString(txt, fmt.Sprintf("--%s=<REDACTED>", header))
	}
	return txt
}

func stripURLSecretsFromFile(file *bespb.File) *bespb.File {
	switch p := file.GetFile().(type) {
	case *bespb.File_Uri:
		p.Uri = stripURLSecrets(p.Uri)
	}
	return file
}

func stripURLSecretsFromFiles(files []*bespb.File) []*bespb.File {
	for index, file := range files {
		files[index] = stripURLSecretsFromFile(file)
	}
	return files
}

func stripRepoURLCredentialsFromBuildMetadata(metadata *bespb.BuildMetadata) {
	for _, repoURLKey := range knownGitRepoURLKeys {
		if val := metadata.Metadata[repoURLKey]; val != "" {
			metadata.Metadata[repoURLKey] = gitutil.StripRepoURLCredentials(val)
		}
	}
	if m, ok := metadata.Metadata[explicitCommandLineName]; ok {
		var commandLine []string
		_ = json.Unmarshal([]byte(m), &commandLine)
		redactCmdLine(commandLine)
		commandLineJSON, _ := json.Marshal(commandLine)
		metadata.Metadata[explicitCommandLineName] = string(commandLineJSON)
	}
}

func stripRepoURLCredentialsFromWorkspaceStatus(status *bespb.WorkspaceStatus) {
	for _, item := range status.Item {
		if item.Value == "" {
			continue
		}
		for _, repoURLKey := range knownGitRepoURLKeys {
			if item.Key == repoURLKey {
				item.Value = gitutil.StripRepoURLCredentials(item.Value)
				break
			}
		}
	}
}

func stripRepoURLCredentialsFromCommandLineOption(option *clpb.Option) {
	// Only strip repo URLs from env var options that point to known Git env
	// vars; other options will be stripped using the regex-based method.
	if !slices.Contains(envVarOptionNames, option.OptionName) {
		return
	}
	for _, repoURLKey := range knownGitRepoURLKeys {
		// assignmentPrefix is a string like "REPO_URL=" or "GIT_URL="
		assignmentPrefix := repoURLKey + envVarSeparator
		if strings.HasPrefix(option.OptionValue, assignmentPrefix) {
			envVarValue := strings.TrimPrefix(option.OptionValue, assignmentPrefix)
			strippedValue := gitutil.StripRepoURLCredentials(envVarValue)
			option.OptionValue = assignmentPrefix + strippedValue
			option.CombinedForm = envVarPrefix + option.OptionName + envVarSeparator + option.OptionValue
			return
		}
	}
}

func filterCommandLineOptions(options []*clpb.Option) []*clpb.Option {
	filtered := []*clpb.Option{}
	for _, option := range options {
		// Remove default_overrides for now since we don't have a use for them (yet)
		// and they may contain sensitive info.
		if option.OptionName == "default_override" {
			continue
		}
		if option.OptionName == "build_metadata" && strings.HasPrefix(option.OptionValue, explicitCommandLineName) {
			continue
		}
		filtered = append(filtered, option)
	}
	return filtered
}

// splitCombinedForm splits a combined form like `--flag_name=value` around
// the first "=", returning "--flag_name", "value".
//
// Notes:
//   - The value may be empty (e.g. `--flag` returns "--flag", "")
//   - If the equals sign is omitted, the flag is treated as above.
//   - The value man contain equals signs (e.g. `--key=Q8s-=2` returns "--key",
//     "Q8s-=2")
//   - The leading dashes are not required and will be omitted if not provided.
func splitCombinedForm(cf string) (string, string) {
	i := strings.Index(cf, "=")
	if i < 0 {
		return cf, ""
	}
	return cf[:i], cf[i+1:]
}

func redactStructuredCommandLine(commandLine *clpb.CommandLine, allowedEnvVars []string) error {
	command := ""
	residualChunks := []*clpb.ChunkList{}

	for _, section := range commandLine.Sections {
		if p, ok := section.SectionType.(*clpb.CommandLineSection_OptionList); ok {
			p.OptionList.Option = filterCommandLineOptions(p.OptionList.Option)
			for _, option := range p.OptionList.Option {
				// Strip URL secrets. Strip git URLs explicitly first, since
				// gitutil.StripRepoURLCredentials is URL-aware. Then fall back to
				// regex-based stripping.
				stripRepoURLCredentialsFromCommandLineOption(option)

				if isKnownMultiFlag(option.OptionName) {
					option.OptionValue = stripUrlSecretsFromMultiFlagValue(option.OptionValue)
					option.CombinedForm = stripUrlSecretsFromMultiFlag(option.CombinedForm)
				} else {
					option.OptionValue = stripURLSecrets(option.OptionValue)
					option.CombinedForm = stripUrlSecretsFromFlag(option.CombinedForm)
				}

				// Redact remote header values
				for _, name := range headerOptionNames {
					if option.OptionName == name {
						option.OptionValue = redactedPlaceholder
						option.CombinedForm = envVarPrefix + option.OptionName + envVarSeparator + redactedPlaceholder
					}
				}

				// Redact non-allowed env vars
				if slices.Contains(envVarOptionNames, option.OptionName) {
					parts := strings.Split(option.OptionValue, envVarSeparator)
					if len(parts) == 0 || isAllowedEnvVar(parts[0], allowedEnvVars) {
						continue
					}
					option.OptionValue = parts[0] + envVarSeparator + redactedPlaceholder
					option.CombinedForm = envVarPrefix + option.OptionName + envVarSeparator + parts[0] + envVarSeparator + redactedPlaceholder
				}

				// Redact sensitive platform props
				if option.OptionName == "remote_default_exec_properties" {
					for _, propName := range redactedPlatformProps {
						if strings.HasPrefix(option.OptionValue, propName+"=") {
							option.OptionValue = propName + "=<REDACTED>"
							option.CombinedForm = "--remote_default_exec_properties=" + propName + "=<REDACTED>"
						}
					}
				}

				// Redact bazel sub command (for remote runners)
				if option.OptionName == "bazel_sub_command" {
					redactedCmd, err := RedactCommand(option.OptionValue)
					if err != nil {
						return status.WrapError(err, "redact command")
					}
					option.OptionValue = redactedCmd
				}

				if option.OptionName == "serialized_action" {
					decodedAction, err := base64.StdEncoding.DecodeString(option.OptionValue)
					if err != nil {
						return status.WrapError(err, "decode serialized action")
					}
					redactedAction := RedactText(string(decodedAction))
					option.OptionValue = base64.StdEncoding.EncodeToString([]byte(redactedAction))
				}
			}
			continue
		}

		if p, ok := section.SectionType.(*clpb.CommandLineSection_ChunkList); ok {
			if section.SectionLabel == "command" {
				command = p.ChunkList.Chunk[0]
			}

			if section.SectionLabel == "residual" {
				residualChunks = append(residualChunks, p.ChunkList)
			}
		}
	}

	if command == "run" {
		// Redact residual chunks when the BEP was generated by a "bazel run" command.
		// In other cases, the residual chunks are usually build targets and should not be redacted.
		for _, cl := range residualChunks {
			redactResidualChunkList(cl)
		}
	}
	return nil
}

func redactResidualChunkList(chunkList *clpb.ChunkList) {
	filteredChunks := make([]string, 0, len(chunkList.Chunk))

	filterNext := false
	for _, c := range chunkList.Chunk {
		if filterNext {
			filterNext = false
			filteredChunks = append(filteredChunks, redactedPlaceholder)
			continue
		}

		if residualSecretRegex.MatchString(c) {
			if strings.Contains(c, "=") {
				chunks := strings.SplitN(c, "=", 2)
				filteredChunks = append(filteredChunks, chunks[0]+"="+redactedPlaceholder)
				continue
			}
			filteredChunks = append(filteredChunks, c)
			filterNext = true
			continue
		}

		filteredChunks = append(filteredChunks, c)
	}

	chunkList.Chunk = filteredChunks
}

func isAllowedEnvVar(variableName string, allowedEnvVars []string) bool {
	lowercaseVariableName := strings.ToLower(variableName)
	for _, allowed := range allowedEnvVars {
		lowercaseAllowed := strings.ToLower(allowed)
		if allowed == "*" || lowercaseVariableName == lowercaseAllowed {
			return true
		}
		isWildCard := strings.HasSuffix(allowed, "*")
		allowedPrefix := strings.ReplaceAll(lowercaseAllowed, "*", "")
		if isWildCard && strings.HasPrefix(lowercaseVariableName, allowedPrefix) {
			return true
		}
	}
	return false
}

// parseAllowedEnv looks for an option like "--build_metadata='ALLOW_ENV=A,B,C'"
// and returns a slice of the comma-separated values specified in the value of
// ALLOW_ENV -- in this example, it would return {"A", "B", "C"}.
func parseAllowedEnv(optionsDescription string) []string {
	options := strings.Split(optionsDescription, " ")
	for _, option := range options {
		if !strings.HasPrefix(option, buildMetadataOptionPrefix) {
			continue
		}
		optionValue := strings.TrimPrefix(option, buildMetadataOptionPrefix)
		if strings.HasPrefix(optionValue, "'") && strings.HasSuffix(optionValue, "'") {
			optionValue = strings.TrimPrefix(optionValue, "'")
			optionValue = strings.TrimSuffix(optionValue, "'")
		}
		if !strings.HasPrefix(optionValue, allowEnvPrefix) {
			continue
		}
		envVarValue := strings.TrimPrefix(optionValue, allowEnvPrefix)
		return strings.Split(envVarValue, allowEnvListSeparator)
	}

	return []string{}
}

// StreamingRedactor processes a stream of build events and redacts them as they are
// received by the event handler.
type StreamingRedactor struct {
	env            environment.Env
	allowedEnvVars []string
}

func NewStreamingRedactor(env environment.Env) *StreamingRedactor {
	return &StreamingRedactor{
		env:            env,
		allowedEnvVars: defaultAllowedEnvVars,
	}
}

func (r *StreamingRedactor) RedactMetadata(event *bespb.BuildEvent) error {
	switch p := event.Payload.(type) {
	case *bespb.BuildEvent_Progress:
		{
		}
	case *bespb.BuildEvent_Aborted:
		{
		}
	case *bespb.BuildEvent_Started:
		{
			r.allowedEnvVars = append(r.allowedEnvVars, parseAllowedEnv(p.Started.OptionsDescription)...)
			// Redact the whole options description to avoid having to parse individual
			// options. The StructuredCommandLine should contain all of these options
			// anyway.
			p.Started.OptionsDescription = "<REDACTED>"
		}
	case *bespb.BuildEvent_UnstructuredCommandLine:
		{
			// Clear the unstructured command line so we don't have to redact it.
			p.UnstructuredCommandLine.Args = []string{}
		}
	case *bespb.BuildEvent_StructuredCommandLine:
		{
			if err := redactStructuredCommandLine(p.StructuredCommandLine, r.allowedEnvVars); err != nil {
				return err
			}
		}
	case *bespb.BuildEvent_OptionsParsed:
		{
			redactCmdLine(p.OptionsParsed.CmdLine)
			redactCmdLine(p.OptionsParsed.ExplicitCmdLine)
		}
	case *bespb.BuildEvent_WorkspaceStatus:
		{
			stripRepoURLCredentialsFromWorkspaceStatus(p.WorkspaceStatus)
		}
	case *bespb.BuildEvent_Fetch:
		{
		}
	case *bespb.BuildEvent_Configuration:
		{
		}
	case *bespb.BuildEvent_Expanded:
		{
		}
	case *bespb.BuildEvent_Configured:
		{
		}
	case *bespb.BuildEvent_Action:
		{
			p.Action.Stdout = stripURLSecretsFromFile(p.Action.Stdout)
			p.Action.Stderr = stripURLSecretsFromFile(p.Action.Stderr)
			p.Action.PrimaryOutput = stripURLSecretsFromFile(p.Action.PrimaryOutput)
			p.Action.ActionMetadataLogs = stripURLSecretsFromFiles(p.Action.ActionMetadataLogs)
		}
	case *bespb.BuildEvent_NamedSetOfFiles:
		{
			p.NamedSetOfFiles.Files = stripURLSecretsFromFiles(p.NamedSetOfFiles.Files)
		}
	case *bespb.BuildEvent_Completed:
		{
			p.Completed.DirectoryOutput = stripURLSecretsFromFiles(p.Completed.DirectoryOutput)
		}
	case *bespb.BuildEvent_TestResult:
		{
			p.TestResult.TestActionOutput = stripURLSecretsFromFiles(p.TestResult.TestActionOutput)
		}
	case *bespb.BuildEvent_TestSummary:
		{
			p.TestSummary.Passed = stripURLSecretsFromFiles(p.TestSummary.Passed)
			p.TestSummary.Failed = stripURLSecretsFromFiles(p.TestSummary.Failed)
		}
	case *bespb.BuildEvent_Finished:
		{
		}
	case *bespb.BuildEvent_BuildToolLogs:
		{
			p.BuildToolLogs.Log = stripURLSecretsFromFiles(p.BuildToolLogs.Log)
		}
	case *bespb.BuildEvent_BuildMetrics:
		{
		}
	case *bespb.BuildEvent_WorkspaceInfo:
		{
		}
	case *bespb.BuildEvent_BuildMetadata:
		{
			stripRepoURLCredentialsFromBuildMetadata(p.BuildMetadata)
		}
	case *bespb.BuildEvent_ConvenienceSymlinksIdentified:
		{
		}
	case *bespb.BuildEvent_WorkflowConfigured:
		{
			for _, m := range p.WorkflowConfigured.Invocation {
				redactedCmd, err := RedactCommand(m.BazelCommand)
				if err != nil {
					return status.WrapError(err, "redact command")
				}
				m.BazelCommand = redactedCmd
			}
		}
	case *bespb.BuildEvent_ChildInvocationsConfigured:
		{
			for _, m := range p.ChildInvocationsConfigured.Invocation {
				redactedCmd, err := RedactCommand(m.BazelCommand)
				if err != nil {
					return status.WrapError(err, "redact command")
				}
				m.BazelCommand = redactedCmd
			}
		}
	}
	return nil
}

// Warning: This command won't parse arbitrary text correctly (Ex. if there's an
// unterminated quote, it will return an error). Use `RedactText` to redact
// more arbitary strings.
func RedactCommand(cmd string) (string, error) {
	cmdTokens, err := shlex.Split(cmd)
	if err != nil {
		return "", status.WrapError(err, "split command")
	}
	redactCmdLine(cmdTokens)
	return strings.Join(cmdTokens, " "), nil
}

func (r *StreamingRedactor) RedactAPIKey(ctx context.Context, event *bespb.BuildEvent) error {
	apiKey, ok := ctx.Value("x-buildbuddy-api-key").(string)
	if !ok || apiKey == "" {
		return nil
	}
	// Traverse the event with reflection and redact any values that may contain
	// an API key.
	// Note: a find-and-replace on the prototext representation would be a much
	// simpler option, but it also has much higher CPU/memory overhead.
	reflectRedactAPIKey(reflect.ValueOf(event), apiKey)
	return nil
}

func reflectRedactAPIKey(value reflect.Value, apiKey string) *reflect.Value {
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface:
		if !value.IsValid() {
			return nil
		}
		redactedValue := reflectRedactAPIKey(value.Elem(), apiKey)
		if redactedValue != nil {
			value.Elem().Set(*redactedValue)
		}
		return nil
	case reflect.Struct:
		for i := 0; i < value.NumField(); i += 1 {
			f := value.Field(i)
			if !f.CanSet() {
				// Skip private fields
				continue
			}
			redactedValue := reflectRedactAPIKey(f, apiKey)
			if redactedValue != nil {
				f.Set(*redactedValue)
			}
		}
		return nil
	case reflect.Slice:
		for i := 0; i < value.Len(); i += 1 {
			f := value.Index(i)
			redactedValue := reflectRedactAPIKey(f, apiKey)
			if redactedValue != nil {
				f.Set(*redactedValue)
			}
		}
		return nil
	case reflect.Map:
		type replacement struct{ originalKey, redactedKey, redactedValue reflect.Value }
		var replacements []replacement
		for _, key := range value.MapKeys() {
			redactedKey := reflectRedactAPIKey(key, apiKey)
			redactedValue := reflectRedactAPIKey(value.MapIndex(key), apiKey)
			if redactedValue != nil {
				value.SetMapIndex(key, *redactedValue)
			}
			if redactedKey != nil {
				replacements = append(replacements, replacement{key, *redactedKey, value.MapIndex(key)})
			}
		}
		for _, r := range replacements {
			// Delete old key
			value.SetMapIndex(r.originalKey, reflect.Value{})
			// Replace with redacted entry
			value.SetMapIndex(r.redactedKey, r.redactedValue)
		}
		return nil
	case reflect.String:
		s := value.String()
		if !strings.Contains(s, apiKey) {
			return nil
		}
		redacted := strings.ReplaceAll(s, apiKey, "<REDACTED>")
		redactedValue := reflect.ValueOf(redacted)
		// Return the redacted string back to the caller so that it can update
		// the containing element with the redacted string.
		return &redactedValue
	default:
		// Not a string or a type that can contain a string; nothing to redact.
		return nil
	}
}

func (r *StreamingRedactor) RedactAPIKeysWithSlowRegexp(ctx context.Context, event *bespb.BuildEvent) error {
	eventBytes, err := prototext.Marshal(event)
	if err != nil {
		return err
	}
	txt := string(eventBytes)
	txt = redactBuildBuddyAPIKeys(txt)
	if contextKey, ok := ctx.Value("x-buildbuddy-api-key").(string); ok {
		txt = strings.ReplaceAll(txt, contextKey, "<REDACTED>")
	}

	return prototext.Unmarshal([]byte(txt), event)
}
