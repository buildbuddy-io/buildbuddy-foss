import { AlertCircle, AlertTriangle, HelpCircle } from "lucide-react";
import React from "react";
import { execution_stats } from "../../proto/execution_stats_ts_proto";
import { grp } from "../../proto/group_ts_proto";
import { User } from "../auth/user";
import capabilities from "../capabilities/capabilities";
import { TextLink } from "../components/link/link";
import { bytes as formatBytes } from "../format/format";
import InvocationModel from "./invocation_model";

interface Props {
  suggestions: Suggestion[];
  /**
   * Whether this is being displayed from the overview page.
   * Instead of showing all suggestions, this will show a summary
   * and a link to the suggestions tab.
   */
  overview?: boolean;
  user?: User;
}

interface Suggestion {
  level: SuggestionLevel;
  message: React.ReactNode;
  reason: React.ReactNode;
}

interface MatchParams {
  model: InvocationModel;
  buildLogs: string;
  runnerExecution?: execution_stats.Execution;
}

export enum SuggestionLevel {
  INFO,
  WARNING,
  ERROR,
}

/** Given some data about an invocation, optionally returns a suggestion. */
type SuggestionMatcher = (params: MatchParams) => Suggestion | null;

// Recommended --execution_filter values for iOS builds.
const IOS_EXECUTION_FILTER = {
  noRemote: [
    "AppleLipo",
    "BitcodeSymbolsCopy",
    "BundleApp",
    "BundleTreeApp",
    "DsymDwarf",
    "DsymLipo",
    "GenerateAppleSymbolsFile",
    "ObjcBinarySymbolStrip",
    "CppArchive",
    "CppLink",
    "ObjcLink",
    "ProcessAndSign",
    "SignBinary",
    "SwiftArchive",
    "SwiftStdlibCopy",
  ],
  noRemoteExec: ["BundleResources", "ImportedDynamicFrameworkProcessor"],
};

export const getTimingDataSuggestion: SuggestionMatcher = ({ model }) => {
  if (!capabilities.config.expandedSuggestionsEnabled) return null;

  const hasRemoteTimingProfile = model.buildToolLogs?.log.find((log) => log.uri?.startsWith("bytestream://"));
  if (!hasRemoteTimingProfile) return null;

  const recommendedOptions: Record<string, boolean> = {
    slim_profile: false,
    experimental_profile_include_target_label: true,
    experimental_profile_include_primary_output: true,
  };

  const missingFlags = Object.keys(recommendedOptions).filter((flag) => !model.optionsMap.get(flag));
  if (!missingFlags.length) return null;

  return {
    level: SuggestionLevel.INFO,
    message: (
      <>
        <div className="details">
          For a more detailed timing profile, try using these flags:{" "}
          {missingFlags.map((flag) => (
            <>
              <BazelFlag>{`--${recommendedOptions[flag] ? "" : "no"}${flag}`}</BazelFlag>{" "}
            </>
          ))}
        </div>
      </>
    ),
    reason: <>Shown because these flags are neither enabled nor explicitly disabled.</>,
  };
};

// TODO(siggisim): server side suggestion storing, parsing, and fetching.
const matchers: SuggestionMatcher[] = [
  buildLogRegex({
    level: SuggestionLevel.ERROR,
    regex: /stat \/usr\/bin\/gcc: no such file or directory/,
    message: (
      <>
        It looks like the C toolchains aren't configured properly for this invocation, and remote build execution is
        enabled. For more information about configuring toolchains, see the{" "}
        <a href="https://www.buildbuddy.io/docs/rbe-setup" target="_blank">
          BuildBuddy RBE Setup documentation
        </a>
        .
      </>
    ),
  }),
  buildLogRegex({
    level: SuggestionLevel.ERROR,
    regex: /exec user process caused "exec format error"/,
    message: (
      <>
        It looks like the architecture of the host machine does not match the architecture of the remote execution
        workers. This is likely because Bazel is running on a Mac, but only Linux remote executors are registered in the
        remote execution cluster. Try running Bazel on a Linux host or in a Docker container.
      </>
    ),
  }),
  buildLogRegex({
    level: SuggestionLevel.ERROR,
    regex: /rpc error: code = Unavailable desc = No registered executors./,
    message: (
      <>
        It looks like no executors are registered for the configured platform. This is likely because Bazel is running
        on a Mac, but only Linux remote executors are registered in the remote execution cluster. Try running Bazel on a
        Linux host or in a Docker container.
      </>
    ),
  }),
  ({ model, buildLogs }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!model.optionsMap.get("remote_cache") && !model.optionsMap.get("remote_executor")) return null;
    if (!buildLogs.includes("DEADLINE_EXCEEDED")) return null;
    if (model.optionsMap.get("remote_timeout") && Number(model.optionsMap.get("remote_timeout")) >= 600) return null;
    if (!model.isComplete() || model.invocation.success) return null;

    return {
      level: SuggestionLevel.ERROR,
      message: (
        <>
          A "deadline exceeded" error was encountered, possibly due to large artifacts being downloaded or uploaded. If
          you see this often, consider setting a higher value of <BazelFlag>--remote_timeout</BazelFlag>. We recommend a
          value of 600 (10 minutes).
        </>
      ),
      reason: (
        <>
          Shown because this build is cache-enabled, has an effective{" "}
          <span className="inline-code">remote_timeout</span> less than 10 minutes, and failed with a log message
          containing "DEADLINE_EXCEEDED"
        </>
      ),
    };
  },
  // Show warning if VM is low on disk/memory.
  ({ model, runnerExecution, buildLogs }) => {
    if (!runnerExecution) return null;
    // TODO: show for remote bazel as well one we have a way to configure the
    // runner via buildbuddy.yaml?
    if (!model.isWorkflowInvocation()) return null;

    let rootDiskUsage = 0.0;
    let rootDiskBytesUsed = 0;
    let rootDiskBytesTotal = 0;
    // Get execution metadata from the cached ExecutionResponse,
    // since we don't store this in the DB currently.
    const executionMetadata = runnerExecution?.executeResponse?.result?.executionMetadata;
    if (!executionMetadata) return null;

    for (const fs of executionMetadata.usageStats?.peakFileSystemUsage ?? []) {
      if (fs.target === "/" && Number(fs.totalBytes) !== 0) {
        rootDiskUsage = Number(fs.usedBytes) / Number(fs.totalBytes);
        rootDiskBytesUsed = Number(fs.usedBytes);
        rootDiskBytesTotal = Number(fs.totalBytes);
      }
    }
    const memoryUsedBytes = Number(executionMetadata?.usageStats?.peakMemoryBytes);
    const memoryTotalBytes = Number(executionMetadata?.estimatedTaskSize?.estimatedMemoryBytes);
    let memoryUsage = 0.0;
    if (memoryTotalBytes !== 0) {
      memoryUsage = memoryUsedBytes / memoryTotalBytes;
    }

    const lowResources: string[] = [];
    const yamlSuggestions: string[] = [];
    if (rootDiskUsage > 0.98) {
      lowResources.push(`disk (${formatBytes(rootDiskBytesUsed)} used of ${formatBytes(rootDiskBytesTotal)} total)`);
      // Conservatively suggest adding 5GB for now.
      yamlSuggestions.push(`disk: ${Math.floor((rootDiskBytesTotal + 5e9) / 1e9)}GB`);
    }
    if (memoryUsage > 0.98) {
      lowResources.push(`memory (${formatBytes(memoryUsedBytes)} used of ${formatBytes(memoryTotalBytes)} total)`);
      // Conservatively suggest adding 2GB for now.
      yamlSuggestions.push(`memory: ${Math.floor((memoryTotalBytes + 2e9) / 1e9)}GB`);
    }

    if (!lowResources.length) return null;

    const hasNoSpaceError = buildLogs.match(/[Nn]o space left on device/);
    return {
      level: hasNoSpaceError ? SuggestionLevel.ERROR : SuggestionLevel.WARNING,
      message: (
        <>
          <p>
            The VM that executed this invocation is low on <InlineProseList items={lowResources} />.
          </p>
          <p>
            To increase the available resources, set <span className="inline-code">resource_requests</span> in{" "}
            <TextLink href="https://buildbuddy.io/docs/workflows-config">
              <span className="inline-code">buildbuddy.yaml</span>
            </TextLink>
            :
          </p>
          <code>
            <pre>
              {`actions:
  - name: ${model.workflowConfigured?.actionName || "..."}
    # ...
    resource_requests:
${yamlSuggestions.map((s) => `      ${s}`).join("\n")}`}
            </pre>
          </code>
        </>
      ),
      reason: (
        <>
          Shown because the{" "}
          <TextLink href={`/invocation/${model.getInvocationId()}#execution`}>runner execution</TextLink> reported high
          resource usage{hasNoSpaceError && ' and the build log contains "no space left on device"'}.
        </>
      ),
    };
  },
  // Suggest recommended flags for `bazel coverage` when using RBE
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (model.getCommand() !== "coverage") return null;
    if (!model.optionsMap.get("remote_executor")) return null;
    if (
      model.optionsMap.has("experimental_split_coverage_postprocessing") ||
      model.optionsMap.has("experimental_fetch_all_coverage_outputs")
    ) {
      return null;
    }

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          <p>
            To ensure test coverage outputs are downloaded when using remote execution, you may need to set{" "}
            <BazelFlag>--experimental_split_coverage_postprocessing</BazelFlag> and{" "}
            <BazelFlag>--experimental_fetch_all_coverage_outputs</BazelFlag>.
          </p>
          <p>
            Some other flags may be necessary as well. See{" "}
            <TextLink href="https://bazel.build/configure/coverage#remote-execution">
              Bazel coverage remote execution configuration
            </TextLink>{" "}
            and <TextLink href="https://github.com/bazelbuild/bazel/issues/4685">bazelbuild/bazel#4685</TextLink> for
            more information.
          </p>
        </>
      ),
      reason: (
        <>
          Shown because this coverage invocation is using remote execution, but does not explicitly set{" "}
          <BazelFlag>--experimental_split_coverage_postprocessing</BazelFlag> or{" "}
          <BazelFlag>--experimental_fetch_all_coverage_outputs</BazelFlag>.
        </>
      ),
    };
  },
  // Suggest remote.buildbuddy.io instead of cloud.buildbuddy.io
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    // remote.buildbuddy.io doesn't support cert-based auth
    if (model.optionsMap.get("tls_client_certificate")) return null;
    const flags = [];
    for (const flag of ["bes_backend", "remote_cache", "remote_executor"]) {
      const value = model.optionsMap.get(flag) || "";
      if (value === "cloud.buildbuddy.io" || value === "grpcs://cloud.buildbuddy.io") {
        flags.push(flag);
      }
    }
    if (!flags.length) return null;
    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          This build is using the old BuildBuddy endpoint, <span className="inline-code">cloud.buildbuddy.io</span>.
          Migrate to <span className="inline-code">remote.buildbuddy.io</span> for improved performance.
        </>
      ),
      reason: (
        <>
          Shown because this build has the effective flag{flags.length === 1 ? "" : "s"}{" "}
          {flags.map((flag, i) => (
            <>
              <span className="inline-code">
                --{flag}={model.optionsMap.get(flag)}
              </span>
              {i < flags.length - 1 && " "}
            </>
          ))}
        </>
      ),
    };
  },
  // Suggest using cache compression
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (model.optionsMap.get("remote_cache_compression")) return null;
    if (model.optionsMap.get("experimental_remote_cache_compression")) return null;
    if (!model.optionsMap.get("remote_cache") && !model.optionsMap.get("remote_executor")) return null;

    const version = getBazelVersion(model);
    // Bazel pre-v5 doesn't support compression.
    if (version === null || version.major < 5) return null;

    const flag = version.major >= 7 ? "--experimental_remote_cache_compression" : "--remote_cache_compression";

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider adding the Bazel flag <BazelFlag>{flag}</BazelFlag> to improve remote cache throughput.
        </>
      ),
      reason: (
        <>
          Shown because this build is cache-enabled but <span className="inline-code">{flag}</span> is neither enabled
          nor explicitly disabled.
        </>
      ),
    };
  },
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (model.optionsMap.get("experimental_remote_cache_compression_threshold")) return null;
    if (
      !model.optionsMap.get("remote_cache_compression") &&
      !model.optionsMap.get("experimental_remote_cache_compression")
    )
      return null;
    if (!model.optionsMap.get("remote_cache") && !model.optionsMap.get("remote_executor")) return null;

    const version = getBazelVersion(model);
    // threshold flag is available from Bazel 7.1 forward
    if (version === null || version.major < 7 || version.minor < 1) return null;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider adding the Bazel flag <BazelFlag>--experimental_remote_cache_compression_threshold=100</BazelFlag> to
          avoid inflating blobs smaller than 100 bytes with ZSTD compression.
        </>
      ),
      reason: (
        <>
          Shown because this build is cache-enabled with <span className="inline-code">--remote_cache_compression</span>{" "}
          set without <span className="inline-code">--experimental_remote_cache_compression_threshold</span> set.
        </>
      ),
    };
  },
  // Suggest using --jobs
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!model.optionsMap.get("remote_executor")) return null;
    if (model.optionsMap.get("jobs")) return null;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider setting the Bazel flag <BazelFlag>--jobs</BazelFlag> to allow more actions to execute in parallel,
          which can significantly improve remote execution performance. We recommend starting with{" "}
          <span className="inline-code">--jobs=50</span> and working your way up.
        </>
      ),
      reason: <>Shown because this build has remote execution enabled, but a jobs count is not configured.</>,
    };
  },
  // Suggest --experimental_remote_build_event_upload=minimal
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!model.optionsMap.get("remote_cache")) return null;
    if (model.optionsMap.get("remote_build_event_upload")) return null;
    if (model.optionsMap.get("experimental_remote_build_event_upload")) return null;
    const version = getBazelVersion(model);
    // Bazel pre-v6 doesn't support --experimental_remote_build_event_upload=minimal, and Bazel post-v6 default to the
    // correct setting
    if (version === null || version.major != 6) return null;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider setting the Bazel flag <BazelFlag>--experimental_remote_build_event_upload=minimal</BazelFlag> to
          reduce the number of unnecessary cache uploads that Bazel might perform during a build.
        </>
      ),
      reason: (
        <>
          Shown because this build has remote cache enabled, and{" "}
          <span className="inline-code">--experimental_remote_build_event_upload</span> is not explicitly set.
        </>
      ),
    };
  },
  // Suggest timing profile flags
  getTimingDataSuggestion,
  // Suggest --nolegacy_important_outputs
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    const version = getBazelVersion(model);
    if (version === null || version.major >= 8) return null;

    if (model.optionsMap.get("legacy_important_outputs")) return null;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider adding the Bazel flag <BazelFlag>--nolegacy_important_outputs</BazelFlag>, which can significantly
          reduce the payload size of the uploaded build event stream by eliminating duplicate file references.
        </>
      ),
      reason: (
        <>
          Shown because
          <span className="inline-code">--legacy_important_outputs</span> is not explicitly set.
        </>
      ),
    };
  },
  // Suggest modify_execution_info for iOS builds
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!model.optionsMap.get("remote_cache") && !model.optionsMap.get("remote_executor")) return null;
    if (model.optionsMap.get("modify_execution_info")) return null;

    const allIOSMnemonics = new Set(Object.values(IOS_EXECUTION_FILTER).flat());
    const triggeredMnemonics: React.ReactNode[] = [];
    for (const action of model.buildMetrics?.actionSummary?.actionData || []) {
      if (allIOSMnemonics.has(action.mnemonic)) {
        triggeredMnemonics.push(<span className="inline-code">{action.mnemonic}</span>);
      }
    }
    // Need a few mnemonics to match so we can be more certain this is an iOS build.
    if (triggeredMnemonics.length <= 2) return null;

    const noRemoteExpr = IOS_EXECUTION_FILTER.noRemote.join("|");
    const noRemoteExecExpr = IOS_EXECUTION_FILTER.noRemoteExec.join("|");
    const recommendedFlag = `--modify_execution_info=^(${noRemoteExpr})$=+no-remote,^(${noRemoteExecExpr})$=+no-remote-exec`;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          <div>
            Consider using the Bazel flag <BazelFlag>--modify_execution_info</BazelFlag> to selectively disable remote
            capabilities for certain build actions, which can improve overall build performance. For iOS builds, we
            recommend:
          </div>
          <code>{recommendedFlag}</code>
        </>
      ),
      reason: (
        <>
          Shown because this build is not configured with <span className="inline-code">--modify_execution_info</span>{" "}
          and it looks like an iOS build based on the executed action{triggeredMnemonics.length === 1 ? "" : "s"}{" "}
          <InlineProseList items={triggeredMnemonics} />
        </>
      ),
    };
  },
  // Suggest experimental_remote_cache_async
  ({ model }) => {
    // TODO(bduffany): This flag potentially causes cache poisoning; re-enable
    // with min Bazel version check once issues are fixed.
    return null;

    /* No clue why, but strict TS gets broken by dead code, so this is commented out.
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!model.optionsMap.get("remote_cache") && !model.optionsMap.get("remote_executor")) return null;
    if (model.optionsMap.get("experimental_remote_cache_async")) return null;
    const version = getBazelMajorVersion(model);
    if (version === null || version < 5) return null;

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Consider enabling <BazelFlag>--experimental_remote_cache_async</BazelFlag> to improve remote cache
          performance.
        </>
      ),
      reason: (
        <>
          Shown because this build is cache-enabled but{" "}
          <span className="inline-code">--experimental_remote_cache_async</span> is neither enabled nor explicitly
          disabled.
        </>
      ),
    };
    */
  },
  // Suggest configuring metadata to enable test grid
  ({ model }) => {
    if (!capabilities.config.expandedSuggestionsEnabled) return null;
    if (!model.isBazelInvocation()) return null;

    if (!capabilities.config.testDashboardEnabled) return null;
    if (model.invocation.role !== "CI") return null;
    if (model.invocation.command !== "test") return null;
    if (model.getCommit() && model.getRepo()) return null;

    const missing = [
      !model.getCommit() && <span className="inline-code">COMMIT_SHA</span>,
      !model.getRepo() && <span className="inline-code">REPO_URL</span>,
    ];

    return {
      level: SuggestionLevel.INFO,
      message: (
        <>
          Add <InlineProseList items={missing} /> metadata to see CI tests in the{" "}
          <TextLink href="/tests/">test dashboard</TextLink> (see the{" "}
          <TextLink href="https://www.buildbuddy.io/docs/guide-metadata/">build metadata guide</TextLink> for help).
        </>
      ),
      reason: (
        <>
          Shown because this invocation was detected to be a CI test, but is missing metadata for{" "}
          <InlineProseList items={missing} />.
        </>
      ),
    };
  },
];

export function getSuggestions({
  model,
  buildLogs,
  runnerExecution,
  user,
}: {
  model: InvocationModel;
  buildLogs: string;
  runnerExecution?: execution_stats.Execution;
  user?: User;
}): Suggestion[] {
  if (!buildLogs || !model || !user) return [];

  const preference = user.selectedGroup.suggestionPreference;
  if (preference === grp.SuggestionPreference.DISABLED) return [];
  if (preference === grp.SuggestionPreference.ADMINS_ONLY && !user.canCall("updateGroup")) {
    return [];
  }

  const suggestions: Suggestion[] = [];
  for (let matcher of matchers) {
    const suggestion = matcher({ buildLogs, model, runnerExecution });
    if (suggestion) suggestions.push(suggestion);
  }
  return suggestions;
}

export default class SuggestionCardComponent extends React.Component<Props> {
  render() {
    const suggestions = this.props.suggestions;
    if (!suggestions.length) return null;

    // When rendering the overview, only show 1 up to one suggestion.
    if (this.props.overview) {
      const suggestion = suggestions[0];
      const hiddenSuggestionsCount = suggestions.length - 1;
      return (
        <div className="card card-suggestion">
          {renderIcon(suggestion.level)}
          <div className="content">
            <div className="title">Suggestion{suggestions.length === 1 ? "" : "s"} from the BuildBuddy Team</div>
            <div className="details">
              <div className="card-suggestion-message">{suggestion.message}</div>
              {hiddenSuggestionsCount > 0 && (
                <div className="suggestions-tab-link">
                  <TextLink href="#suggestions">
                    See {suggestions.length - 1} more suggestion{hiddenSuggestionsCount === 1 ? "" : "s"}
                  </TextLink>
                </div>
              )}
            </div>
          </div>
        </div>
      );
    }

    return (
      <div className="suggestions">
        {suggestions.map((suggestion) => (
          <SuggestionComponent suggestion={suggestion} />
        ))}
        {this.props.user?.canCall("updateGroup") && (
          <TextLink className="settings-link" href="/settings/org/details">
            Suggestion settings
          </TextLink>
        )}
      </div>
    );
  }
}

function renderIcon(level: SuggestionLevel) {
  switch (level) {
    case SuggestionLevel.INFO:
      return <HelpCircle className="icon" />;
    case SuggestionLevel.WARNING:
      return <AlertTriangle className="icon orange" />;
    case SuggestionLevel.ERROR:
      return <AlertCircle className="icon red" />;
  }
}

export interface SuggestionComponentProps {
  suggestion: Suggestion;
}

export function SuggestionComponent({ suggestion }: SuggestionComponentProps) {
  return (
    <div className="card card-suggestion">
      {renderIcon(suggestion.level)}
      <div className="content">
        <div className="details">
          <div className="card-suggestion-message">{suggestion.message}</div>
          <div className="card-suggestion-reason">{suggestion.reason}</div>
        </div>
      </div>
    </div>
  );
}

/** Returns the given suggestion message if the given regex matches the build logs. */
function buildLogRegex({
  level,
  regex,
  message,
}: {
  level: SuggestionLevel;
  regex: RegExp;
  message: React.ReactNode;
}): SuggestionMatcher {
  return ({ buildLogs }) => {
    const matches = buildLogs.match(regex);
    if (!matches) return null;
    const reason = <>Shown because your build log contains "{matches[0]}"</>;
    return { level, message, reason };
  };
}

function BazelFlag({ children }: { children: string }) {
  let flag = children.split("=")[0] || "";
  if (flag.startsWith("--no")) {
    flag = "--" + flag.substring("--no".length);
  }
  return (
    <TextLink
      className="inline-code bazel-flag"
      href={`https://docs.bazel.build/versions/main/command-line-reference.html#flag${flag}`}>
      {children}
    </TextLink>
  );
}

/**
 * Renders a list of items separated by commas, with "and" preceding the final item.
 * For convenience, falsy list items are filtered out.
 */
function InlineProseList({ items }: { items: React.ReactNode[] }) {
  items = items.filter((item) => item);
  const out = [];
  for (let i = 0; i < items.length; i++) {
    out.push(items[i]);

    if (i === items.length - 1) break;

    if (i === items.length - 2) {
      if (items.length === 2) {
        out.push(" and ");
      } else {
        out.push(", and ");
      }
    } else {
      out.push(", ");
    }
  }
  return <>{out}</>;
}

// getBazelVersion returns the major and minor version of Bazel from BES event.
//
// The version could contain rc version in the patch number, such as "7.2.1rc1".
function getBazelVersion(model: InvocationModel): { major: number; minor: number } | null {
  const version = model.started?.buildToolVersion;
  if (!version) return null;
  const segments = version.split(".").map(Number);
  if (segments.length < 2) return null;
  if (segments.slice(0, 2).some(isNaN)) return null;
  return { major: segments[0], minor: segments[1] };
}
