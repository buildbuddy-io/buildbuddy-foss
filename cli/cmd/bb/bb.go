package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/buildbuddy-io/buildbuddy/cli/arg"
	"github.com/buildbuddy-io/buildbuddy/cli/bazelisk"
	"github.com/buildbuddy-io/buildbuddy/cli/cli_command"
	"github.com/buildbuddy-io/buildbuddy/cli/help"
	"github.com/buildbuddy-io/buildbuddy/cli/log"
	"github.com/buildbuddy-io/buildbuddy/cli/metadata"
	"github.com/buildbuddy-io/buildbuddy/cli/parser"
	"github.com/buildbuddy-io/buildbuddy/cli/picker"
	"github.com/buildbuddy-io/buildbuddy/cli/plugin"
	"github.com/buildbuddy-io/buildbuddy/cli/runscript"
	"github.com/buildbuddy-io/buildbuddy/cli/setup"
	"github.com/buildbuddy-io/buildbuddy/cli/watcher"
	"github.com/buildbuddy-io/buildbuddy/server/util/rlimit"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"

	register_cli_commands "github.com/buildbuddy-io/buildbuddy/cli/cli_command/register"
	sidecarmain "github.com/buildbuddy-io/buildbuddy/cli/cmd/sidecar"
)

var (
	// These flags configure the cli at large, and don't apply to any specific
	// cli command
	globalCliFlags = map[string]struct{}{
		// Set to print verbose cli logs
		"verbose": {},
	}
)

const (
	bazelExitCodeTransientBESFailure  = 38
	bazelExitCodePersistentBESFailure = 45
	bazelExitCodeInterrupted          = 8
)

func main() {
	// If we're the sidecar (CLI server) process, run the sidecar instead of the
	// CLI.
	sidecarmain.Handle()

	exitCode, err := run()
	if err != nil {
		log.Fatal(status.Message(err))
	}
	os.Exit(exitCode)
}

func run() (exitCode int, err error) {
	start := time.Now()
	// Record original arguments so we can show them in the UI.
	originalArgs := append([]string{}, os.Args...)

	args := handleGlobalCliFlags(os.Args[1:])

	log.Debugf("CLI started at %s", start)
	log.Debugf("args[0]: %s", os.Args[0])
	log.Debugf("env: BAZEL_REAL=%s", os.Getenv("BAZEL_REAL"))
	log.Debugf("env: BAZELISK_SKIP_WRAPPER=%s", os.Getenv("BAZELISK_SKIP_WRAPPER"))
	log.Debugf("env: USE_BAZEL_VERSION=%s", os.Getenv("USE_BAZEL_VERSION"))
	log.Debugf("env: BB_USE_BAZEL_VERSION=%s", os.Getenv("BB_USE_BAZEL_VERSION"))

	if _, err := bazelisk.ResolveVersion(); err != nil {
		log.Printf("Failed to resolve bazel version: %s", err)
	}

	err = rlimit.MaxRLimit()
	if err != nil {
		log.Printf("Failed to bump open file limits: %s", err)
	}

	// Register all known cli commands so that we can query or iterate them later.
	register_cli_commands.Register()

	// Make sure startup args are always in the format --foo=bar.
	args, err = parser.CanonicalizeArgs(args)
	if err != nil {
		return -1, err
	}

	// Handle help command if applicable.
	exitCode, err = help.HandleHelp(args)
	if err != nil || exitCode >= 0 {
		return exitCode, err
	}

	if c := cli_command.GetCommand(args[0]); c != nil {
		// If the first argument is a cli command, trim it from `args`
		args = args[1:]
		return c.Handler(args)
	}

	// If none of the CLI subcommand handlers were triggered, assume we should
	// handle it as a bazel command.
	return handleBazelCommand(start, args, originalArgs)
}

// handleGlobalCliFlags processes global cli args that don't apply to any specific subcommand
// (--verbose, etc.).
// Returns args with all global cli flags removed
func handleGlobalCliFlags(args []string) []string {
	args, residual := arg.SplitExecutableArgs(args)
	for flag := range globalCliFlags {
		var flagVal string
		flagVal, args = arg.Pop(args, flag)

		// Even if flag is not set and flagVal is "", pass to handlers in case
		// they need to configure a default value
		switch flag {
		case "verbose":
			log.Configure(flagVal)
		}
	}
	return arg.JoinExecutableArgs(args, residual)
}

// handleBazelCommand handles a native bazel command (i.e. commands that are
// directly forwarded to bazel, as opposed to bb cli-specific commands)
//
// originalArgs contains the command as originally typed. We pass it as
// EXPLICIT_COMMAND_LINE metadata to the bazel invocation.
func handleBazelCommand(start time.Time, args []string, originalArgs []string) (int, error) {
	// Maybe run interactively (watching for changes to files).
	if exitCode, err := watcher.Watch(); exitCode >= 0 || err != nil {
		return exitCode, err
	}

	//Prepare a dir for temporary files created by this CLI run
	tempDir, err := os.MkdirTemp("", "buildbuddy-cli-*")
	if err != nil {
		return 1, err
	}

	var scriptPath string
	var exitCode int
	defer func() {
		// Remove tempdir. Need to do this before invoking the run script
		// (if applicable), since the run script will replace this process.
		os.RemoveAll(tempDir)

		// Invoke the run script only if the build succeeded.
		if exitCode == 0 && scriptPath != "" {
			exitCode, err = runscript.Invoke(scriptPath)
		}
	}()

	plugins, bazelArgs, execArgs, sidecar, err := setup.Setup(args, tempDir)
	if err != nil {
		return 1, err
	}

	// Show a picker if the target is omitted. Note: we do this after expanding
	// args, in case a bazelrc specifies the target patterns (e.g. via
	// --target_pattern_file).
	//
	// Note: if there are args after "--" then we never show the picker:
	// - For "build"/"test" commands, the args after "--" are target patterns.
	// - For "run" commands, if there is no target pattern, then the first arg
	//   after "--" is treated as the pattern.
	if len(execArgs) == 0 {
		bazelArgs = picker.HandlePicker(bazelArgs)
	}

	// If this is a `bazel run` command, add a --run_script arg so that
	// we can execute post-bazel plugins between the build and the run step.
	bazelArgs, scriptPath, err = runscript.Configure(bazelArgs)
	if err != nil {
		return 1, err
	}
	// Append metadata just before running bazelisk.
	// Note, this means plugins cannot modify this metadata.
	bazelArgs, err = metadata.AppendBuildMetadata(bazelArgs, originalArgs)
	if err != nil {
		return 1, err
	}

	// Run bazelisk, capturing the original output in a file and allowing
	// plugins to control how the output is rendered to the terminal.
	log.Debugf("bb initialized in %s", time.Since(start))
	outputPath := filepath.Join(tempDir, "bazel.log")
	exitCode, err = plugin.RunBazeliskWithPlugins(
		arg.JoinExecutableArgs(bazelArgs, execArgs),
		outputPath, plugins)
	if err != nil {
		return 1, err
	}

	// If the build was interrupted (Ctrl+C), don't run post-bazel plugins.
	if exitCode == bazelExitCodeInterrupted {
		return exitCode, nil
	}

	// Run plugin post-bazel hooks.
	// Pause the file watcher while these are in progress, so that plugins can
	// apply fixes to files in the workspace without the watcher immediately
	// restarting.
	watcher.Pause()
	defer watcher.Unpause()
	for _, p := range plugins {
		if err := p.PostBazel(outputPath); err != nil {
			return 1, err
		}
	}

	// TODO: Support post-run hooks?
	// e.g. show a desktop notification once a k8s deploy has finished

	// If bazel failed with BES errors, show sidecar warnings/errors to help diagnose.
	if exitCode != 0 && sidecar != nil &&
		(exitCode == bazelExitCodeTransientBESFailure ||
			exitCode == bazelExitCodePersistentBESFailure) {
		sidecar.PrintLogsSince(start)
	}

	return exitCode, nil
}
