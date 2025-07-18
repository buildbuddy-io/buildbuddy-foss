import { CheckCircle, Circle, HelpCircle, PlayCircle, XCircle } from "lucide-react";
import moment from "moment";
import React from "react";
import { Subject } from "rxjs";
import shlex from "shlex";
import { api as api_common } from "../../proto/api/v1/common_ts_proto";
import { build_event_stream } from "../../proto/build_event_stream_ts_proto";
import { cache } from "../../proto/cache_ts_proto";
import { capability } from "../../proto/capability_ts_proto";
import { command_line } from "../../proto/command_line_ts_proto";
import { grp } from "../../proto/group_ts_proto";
import { invocation_status } from "../../proto/invocation_status_ts_proto";
import { invocation } from "../../proto/invocation_ts_proto";
import { build } from "../../proto/remote_execution_ts_proto";
import { resource } from "../../proto/resource_ts_proto";
import { suggestion } from "../../proto/suggestion_ts_proto";
import capabilities from "../capabilities/capabilities";
import { IconType } from "../favicon/favicon";
import format, { formatDate } from "../format/format";
import rpcService from "../service/rpc_service";
import { resourceNameToString } from "../util/cache";
import { exitCode } from "../util/exit_codes";
import { durationToMillisWithFallback, timestampToDateWithFallback } from "../util/proto";

export const CI_RUNNER_ROLE = "CI_RUNNER";
export const HOSTED_BAZEL_ROLE = "HOSTED_BAZEL";

export const InvocationStatus = invocation_status.InvocationStatus;

export default class InvocationModel {
  // The invocations array is guaranteed to have at least 1 invocation.
  readonly invocation: invocation.Invocation;
  readonly cacheStats: cache.CacheStats[];
  scoreCard?: cache.ScoreCard;
  botSuggestions: string[] = [];
  onChange: Subject<void> = new Subject();

  targets: build_event_stream.BuildEvent[] = [];
  succeeded: build_event_stream.BuildEvent[] = [];
  failed: build_event_stream.BuildEvent[] = [];
  skipped: build_event_stream.BuildEvent[] = [];
  fetchEventURLs: string[] = [];
  succeededTest: build_event_stream.BuildEvent[] = [];
  failedTest: build_event_stream.BuildEvent[] = [];
  brokenTest: build_event_stream.BuildEvent[] = [];
  flakyTest: build_event_stream.BuildEvent[] = [];
  timeoutTest: build_event_stream.BuildEvent[] = [];
  structuredCommandLine: command_line.CommandLine[] = [];
  finished?: build_event_stream.BuildFinished;
  aborted: build_event_stream.BuildEvent[] = [];
  failedAction?: build_event_stream.BuildEvent;
  workflowConfigured?: build_event_stream.WorkflowConfigured;
  childInvocationsConfigured: build_event_stream.ChildInvocationsConfigured[] = [];
  childInvocationCompletedByInvocationId = new Map<string, build_event_stream.IChildInvocationCompleted>();
  workspaceStatus?: build_event_stream.WorkspaceStatus;
  configuration?: build_event_stream.Configuration;
  workspaceConfig?: build_event_stream.WorkspaceConfig;
  optionsParsed?: build_event_stream.OptionsParsed;
  unstructuredCommandLine?: build_event_stream.UnstructuredCommandLine;
  started?: build_event_stream.BuildStarted;
  expanded?: build_event_stream.BuildEvent;
  buildMetrics?: build_event_stream.BuildMetrics;
  buildToolLogs?: build_event_stream.BuildToolLogs;

  workspaceStatusMap = new Map<string, string>();
  toolLogMap = new Map<string, string>();
  optionsMap = new Map<string, string>();
  clientEnvMap = new Map<string, string>();
  buildMetadataMap = new Map<string, string>();
  configuredMap = new Map<string, invocation.InvocationEvent>();
  completedMap = new Map<string, invocation.InvocationEvent>();
  skippedMap = new Map<string, invocation.InvocationEvent>();
  testResultMap: Map<string, invocation.InvocationEvent[]> = new Map<string, invocation.InvocationEvent[]>();
  testSummaryMap: Map<string, invocation.InvocationEvent> = new Map<string, invocation.InvocationEvent>();
  actionMap: Map<string, invocation.InvocationEvent[]> = new Map<string, invocation.InvocationEvent[]>();
  rootCauseTargetLabels: Set<String> = new Set<String>();

  private fileSetIDToFilesMap: Map<string, build_event_stream.File[]> = new Map();

  constructor(invocation: invocation.Invocation) {
    this.invocation = invocation;
    this.cacheStats = invocation.cacheStats ? [invocation.cacheStats] : [];
    if (invocation.scoreCard) this.scoreCard = invocation.scoreCard;
    for (let cl of invocation.structuredCommandLine) {
      this.structuredCommandLine.push(cl as command_line.CommandLine);
    }

    for (let event of invocation.event) {
      let buildEvent = event.buildEvent;
      if (!buildEvent) {
        continue;
      }
      if (buildEvent.namedSetOfFiles && buildEvent.id?.namedSet?.id) {
        this.fileSetIDToFilesMap.set(buildEvent.id.namedSet.id, buildEvent.namedSetOfFiles.files);
      }
      if (buildEvent.configured) this.targets.push(buildEvent as build_event_stream.BuildEvent);
      if (buildEvent.configured && buildEvent.id?.targetConfigured?.label) {
        this.configuredMap.set(buildEvent.id.targetConfigured.label, event as invocation.InvocationEvent);
      }
      if (buildEvent.completed && buildEvent.id?.targetCompleted?.label) {
        this.completedMap.set(buildEvent.id.targetCompleted.label, event as invocation.InvocationEvent);
      }
      if (buildEvent.fetch && buildEvent.id?.fetch?.url) {
        this.fetchEventURLs.push(buildEvent.id.fetch.url);
      }
      if (buildEvent.testResult && buildEvent.id?.testResult?.label) {
        let results = this.testResultMap.get(buildEvent.id.testResult.label) || [];
        results.push(event as invocation.InvocationEvent);
        this.testResultMap.set(buildEvent.id.testResult.label, results);
      }
      if (buildEvent.action && buildEvent.id?.actionCompleted?.label) {
        let results = this.actionMap.get(buildEvent.id.actionCompleted.label) || [];
        results.push(event as invocation.InvocationEvent);
        this.actionMap.set(buildEvent.id.actionCompleted.label, results);

        if (buildEvent?.action?.failureDetail?.message && !this.failedAction) {
          this.failedAction = buildEvent;
        }
      }
      if (buildEvent.testSummary && buildEvent.id?.testSummary?.label) {
        this.testSummaryMap.set(buildEvent.id.testSummary.label, event as invocation.InvocationEvent);
      }
      if (buildEvent.started) this.started = buildEvent.started as build_event_stream.BuildStarted;
      if (buildEvent.expanded) this.expanded = buildEvent as build_event_stream.BuildEvent;
      if (buildEvent.finished) this.finished = buildEvent.finished as build_event_stream.BuildFinished;
      if (buildEvent.aborted && buildEvent.aborted.reason == build_event_stream.Aborted.AbortReason.SKIPPED) {
        this.skipped.push(buildEvent as build_event_stream.BuildEvent);
        if (buildEvent.id?.targetCompleted?.label) {
          this.skippedMap.set(buildEvent.id.targetCompleted.label, event as invocation.InvocationEvent);
        }
      } else if (buildEvent.aborted) {
        this.aborted.push(buildEvent);
      }
      if (buildEvent.workspaceStatus) {
        this.workspaceStatus = buildEvent.workspaceStatus as build_event_stream.WorkspaceStatus;
      }
      if (buildEvent.workflowConfigured) {
        this.workflowConfigured = buildEvent.workflowConfigured as build_event_stream.WorkflowConfigured;
      }
      if (buildEvent.childInvocationsConfigured) {
        this.childInvocationsConfigured.push(
          buildEvent.childInvocationsConfigured as build_event_stream.ChildInvocationsConfigured
        );
      }
      if (buildEvent.childInvocationCompleted && buildEvent.id?.childInvocationCompleted?.invocationId) {
        this.childInvocationCompletedByInvocationId.set(
          buildEvent.id.childInvocationCompleted.invocationId,
          buildEvent.childInvocationCompleted
        );
      }
      if (buildEvent.configuration && buildEvent?.id?.configuration?.id != "none") {
        this.configuration = buildEvent.configuration as build_event_stream.Configuration;
      }
      if (buildEvent.workspaceInfo) {
        this.workspaceConfig = buildEvent.workspaceInfo as build_event_stream.WorkspaceConfig;
      }
      if (buildEvent.optionsParsed) {
        this.optionsParsed = buildEvent.optionsParsed as build_event_stream.OptionsParsed;
      }
      if (buildEvent.buildMetrics) {
        this.buildMetrics = buildEvent.buildMetrics as build_event_stream.BuildMetrics;
      }
      if (buildEvent.buildToolLogs) {
        this.buildToolLogs = buildEvent.buildToolLogs as build_event_stream.BuildToolLogs;
      }
      if (buildEvent.unstructuredCommandLine) {
        this.unstructuredCommandLine = buildEvent.unstructuredCommandLine as build_event_stream.UnstructuredCommandLine;
      }
      if (buildEvent.buildMetadata) {
        for (const [key, value] of Object.entries(buildEvent.buildMetadata.metadata || {})) {
          this.buildMetadataMap.set(key, value);
        }
      }
    }
    this.rootCauseTargetLabels = new Set(
      [...this.completedMap.values()]
        .filter((e) => !e.buildEvent?.completed?.success)
        .map((e) => e.buildEvent?.children.filter((child) => child.actionCompleted?.label) || [])
        .flat()
        .map((child) => child.actionCompleted!.label)
    );

    for (let label of this.completedMap.keys()) {
      let buildEvent = this.completedMap.get(label)?.buildEvent;
      let testResult = this.testSummaryMap.get(label)?.buildEvent?.testSummary;
      if (testResult && testResult.overallStatus == build_event_stream.TestStatus.FLAKY) {
        this.flakyTest.push(buildEvent as build_event_stream.BuildEvent);
      } else if (testResult && testResult.overallStatus == build_event_stream.TestStatus.FAILED_TO_BUILD) {
        this.brokenTest.push(buildEvent as build_event_stream.BuildEvent);
      } else if (testResult && testResult.overallStatus == build_event_stream.TestStatus.PASSED) {
        this.succeededTest.push(buildEvent as build_event_stream.BuildEvent);
      } else if (testResult && testResult.overallStatus == build_event_stream.TestStatus.TIMEOUT) {
        this.timeoutTest.push(buildEvent as build_event_stream.BuildEvent);
      } else if (testResult) {
        this.failedTest.push(buildEvent as build_event_stream.BuildEvent);
      }

      if (buildEvent?.completed?.success) {
        this.succeeded.push(buildEvent as build_event_stream.BuildEvent);
      } else {
        this.failed.push(buildEvent as build_event_stream.BuildEvent);
      }
    }

    for (let item of this.workspaceStatus?.item || []) {
      this.workspaceStatusMap.set(item.key, item.value);
    }
    for (let log of this.buildToolLogs?.log || []) {
      this.toolLogMap.set(log.name, new TextDecoder().decode(log.contents || new Uint8Array()));
    }
    // Treat "canonical" command line as the source of truth if present,
    // otherwise just use any command line (e.g. at the time of writing, for
    // workflows we only send "original")
    let commandLine =
      this.structuredCommandLine.find((commandLine) => commandLine.commandLineLabel === "canonical") ??
      this.structuredCommandLine[0];
    if (commandLine) {
      for (let section of commandLine.sections || []) {
        for (let option of section.optionList?.option || []) {
          this.optionsMap.set(option.optionName, option.optionValue);
          if (option.optionName == "client_env") {
            let pair = option.optionValue.split("=");
            if (pair.length == 2) {
              this.clientEnvMap.set(pair[0], pair[1]);
            }
          }
          if (option.optionName == "build_metadata") {
            let parts = option.optionValue.split("=");
            if (parts.length >= 2) {
              let key = parts.shift();
              this.buildMetadataMap.set(key!, parts.join("="));
            }
          }
        }
      }
    }
  }

  getUser() {
    let invocationUser = this.invocation.user;
    if (invocationUser) {
      return invocationUser;
    }
    // TODO: shouldn't the server be populating invocation.user based on these?
    let username = this.workspaceStatusMap.get("BUILD_USER") || this.clientEnvMap.get("USER") || "";
    if (username === "<REDACTED>") {
      return "";
    }
    return username;
  }

  getUserPossessivePrefix() {
    const user = this.getUser();
    if (!user) {
      return "";
    }
    return `${user}'s `;
  }

  /**
   * Returns the group which owns the invocation.
   *
   * If no groups are provided or if the group is not found, null is returned.
   */
  findOwnerGroup(groups: grp.Group[] | undefined) {
    if (!groups?.length) return null;

    return groups.find((group) => group.id === this.invocation.acl?.groupId) || null;
  }

  isAnonymousInvocation(): boolean {
    return this.invocation.acl?.groupId === "";
  }

  hasCacheWriteCapability(): boolean {
    return Boolean(
      this.invocation.createdWithCapabilities?.some(
        (existingCapability) =>
          existingCapability == capability.Capability.CACHE_WRITE ||
          existingCapability == capability.Capability.CAS_WRITE
      )
    );
  }

  getInvocationId(): string {
    return this.invocation.invocationId;
  }

  getAttempt() {
    return this.invocation.attempt;
  }

  getHost() {
    return this.invocation.host || this.workspaceStatusMap.get("BUILD_HOST") || "";
  }

  getTags(): invocation.Invocation.Tag[] {
    return (capabilities.config.tagsUiEnabled && this.invocation.tags) || [];
  }

  booleanCommandLineOption(name: string, defaultValue = false): boolean {
    const rawVal = this.optionsMap.get(name);
    if (rawVal === undefined) return defaultValue;
    return rawVal !== "0";
  }

  stringCommandLineOption(name: string, defaultValue = ""): string {
    const rawVal = this.optionsMap.get(name);
    if (rawVal === undefined) return defaultValue;
    return rawVal;
  }

  isCacheCompressionEnabled() {
    return (
      this.optionsMap.get("experimental_remote_cache_compression") === "1" ||
      this.optionsMap.get("remote_cache_compression") === "1"
    );
  }

  getTargetConfiguredCount() {
    return Number(this.invocation.targetConfiguredCount) || this.targets.length;
  }

  getFailedToBuildCount() {
    return this.invocation.targetGroups.length
      ? this.targetCountForStatus(api_common.v1.Status.FAILED_TO_BUILD)
      : this.brokenTest.length;
  }

  getFailedCount() {
    return this.invocation.targetGroups.length
      ? this.targetCountForStatus(api_common.v1.Status.FAILED)
      : this.failedTest.length;
  }

  getBuiltCount() {
    return this.invocation.targetGroups.length
      ? this.targetCountForStatus(api_common.v1.Status.BUILT)
      : this.succeeded.length;
  }

  getFlakyCount() {
    return this.invocation.targetGroups.length
      ? this.targetCountForStatus(api_common.v1.Status.FLAKY)
      : this.flakyTest.length;
  }

  private targetCountForStatus(status: api_common.v1.Status): number {
    for (const group of this.invocation.targetGroups) {
      if (group.status === status) {
        return Number(group.totalCount);
      }
    }
    return 0;
  }

  getCache() {
    if (!this.optionsMap.get("remote_cache") && !this.optionsMap.get("remote_executor")) {
      return "Cache off";
    }
    if (this.optionsMap.get("remote_upload_local_results") == "0") {
      return "Log upload on";
    }
    if (this.optionsMap.get("remote_download_outputs") == "toplevel") {
      return "Top-level cache";
    }
    if (this.optionsMap.get("remote_download_outputs") == "minimal") {
      return "Minimal cache";
    }
    return "Cache on";
  }

  private parseBytestreamURIPrefix() {
    let prefix = this.optionsMap.get("remote_bytestream_uri_prefix");
    if (!prefix) return null;

    // Strip protocol
    prefix = prefix.replace(/^[a-z]+:\/\//, "");

    // Split host/path
    let idx = prefix.indexOf("/");
    if (idx >= 0) {
      return {
        host: prefix.substring(0, idx),
        pathname: prefix.substring(idx),
      };
    } else {
      return {
        host: prefix,
        pathname: "",
      };
    }
  }

  /**
   * Returns the hostname or hostname:port of the cache address used as the
   * remote cache target for this invocation.
   */
  private getCacheAddress(): string | undefined {
    const prefix = this.parseBytestreamURIPrefix();
    if (prefix) {
      return prefix.host;
    }

    const orderedOptions = ["remote_cache", "remote_executor", "cache_backend", "rbe_backend"];

    for (const optionName of orderedOptions) {
      const option = this.optionsMap.get(optionName);
      if (!option) continue;
      return option.replace("grpc://", "").replace("grpcs://", "");
    }

    return undefined;
  }

  getRemoteInstanceName() {
    return this.optionsMap.get("remote_instance_name") ?? "";
  }

  getCompressor() {
    if (this.isCacheCompressionEnabled()) {
      return build.bazel.remote.execution.v2.Compressor.Value.ZSTD;
    }
    return build.bazel.remote.execution.v2.Compressor.Value.IDENTITY;
  }

  getDigestFunction() {
    const digestFnName = this.optionsMap.get("digest_function");
    if (!digestFnName) {
      return build.bazel.remote.execution.v2.DigestFunction.Value.SHA256;
    }
    const digestFnEnum =
      build.bazel.remote.execution.v2.DigestFunction.Value[
        digestFnName.toUpperCase() as keyof typeof build.bazel.remote.execution.v2.DigestFunction.Value
      ];
    return digestFnEnum || build.bazel.remote.execution.v2.DigestFunction.Value.SHA256;
  }

  /**
   * Returns the base resource name under which all artifacts associated with
   * the invocation are stored.
   */
  private getCacheBaseResourceName(): resource.IResourceName {
    return {
      instanceName: this.getRemoteInstanceName(),
      digestFunction: this.getDigestFunction(),
      // Note: we don't set compressor for now because the UI cannot decompress
      // zstd blobs.
    };
  }

  /**
   * Returns a URL pointing to a CAS artifact associated with this invocation.
   * The returned URL can be used with RpcService to fetch the blob.
   */
  getBytestreamURL(digest: build.bazel.remote.execution.v2.IDigest): string {
    return resourceNameToString(this.getCacheAddress() ?? "", {
      ...this.getCacheBaseResourceName(),
      cacheType: resource.CacheType.CAS,
      digest: new build.bazel.remote.execution.v2.Digest(digest),
    });
  }

  /**
   * Returns a URL pointing to an AC artifact associated with this invocation.
   * The returned URL can be used with RpcService to fetch the ActionResult.
   */
  getActionCacheURL(digest: build.bazel.remote.execution.v2.IDigest): string {
    return resourceNameToString(this.getCacheAddress() ?? "", {
      ...this.getCacheBaseResourceName(),
      cacheType: resource.CacheType.AC,
      digest: new build.bazel.remote.execution.v2.Digest(digest),
    });
  }

  getIsRBEEnabled() {
    return Boolean(this.stringCommandLineOption("remote_executor"));
  }

  getRBE() {
    return this.getIsRBEEnabled() ? "Remote execution on" : "Remote execution off";
  }

  getIsExecutionLogEnabled() {
    return Boolean(
      this.buildToolLogs?.log.find(
        (l: any) =>
          (l.name == "execution.log" || l.name == "execution_log.binpb.zst") && l.uri.startsWith("bytestream://")
      )
    );
  }

  hasCoverage() {
    return (
      this.getCommand() == "coverage" &&
      Boolean(
        this.buildToolLogs?.log.find((l: any) => l.name == "coverage_report.lcov" && l.uri.startsWith("bytestream://"))
      )
    );
  }

  getFetchURLs() {
    return this.fetchEventURLs;
  }

  getRepo() {
    return this.getGithubRepo();
  }

  getCommit() {
    return this.getGithubSHA();
  }

  getBranchName() {
    return this.getGithubBranch();
  }

  getForkRepoURL(): string | undefined {
    return this.buildMetadataMap.get("FORK_REPO_URL");
  }

  getPullRequestNumber(): number | undefined {
    return this.buildMetadataMap.get("PULL_REQUEST_NUMBER")
      ? Number(this.buildMetadataMap.get("PULL_REQUEST_NUMBER"))
      : undefined;
  }

  getGithubUser() {
    return this.clientEnvMap.get("GITHUB_ACTOR");
  }

  getBuildkiteUrl() {
    if (this.clientEnvMap.get("BUILDKITE_BUILD_URL") && this.clientEnvMap.get("BUILDKITE_JOB_ID")) {
      return `${this.clientEnvMap.get("BUILDKITE_BUILD_URL")}#${this.clientEnvMap.get("BUILDKITE_JOB_ID")}`;
    }

    return this.clientEnvMap.get("BUILDKITE_BUILD_URL");
  }

  getGithubRepo(): string {
    return this.invocation.repoUrl;
  }

  getGithubSHA(): string {
    return this.invocation.commitSha;
  }

  getGithubRef() {
    return this.clientEnvMap.get("GITHUB_REF");
  }

  getGithubBranch(): string {
    return this.invocation.branchName;
  }

  getGithubRun() {
    return this.clientEnvMap.get("GITHUB_RUN_ID");
  }

  getGKEProject() {
    return this.clientEnvMap.get("GKE_PROJECT");
  }

  getGKECluster() {
    return this.clientEnvMap.get("GKE_CLUSTER");
  }

  getCommand() {
    return this.started?.command || "build";
  }

  getRole(): string {
    return this.invocation.role;
  }

  isWorkflowInvocation() {
    return this.getRole() === CI_RUNNER_ROLE;
  }

  isHostedBazelInvocation() {
    return this.getRole() === HOSTED_BAZEL_ROLE;
  }

  isBazelInvocation() {
    return !this.isWorkflowInvocation() && !this.isHostedBazelInvocation();
  }

  getTool() {
    if (this.isWorkflowInvocation()) {
      return "BuildBuddy workflow runner";
    }
    if (this.isHostedBazelInvocation()) {
      return "BuildBuddy hosted bazel";
    }
    return `bazel v${this.started?.buildToolVersion} ` + this.started?.command || "build";
  }

  getToolTag() {
    return this.optionsParsed?.toolTag;
  }

  getPattern() {
    return this.getAllPatterns(3);
  }

  getAllPatterns(patternLimit?: number) {
    let patterns =
      this.invocation.pattern ||
      this.expanded?.id?.pattern?.pattern ||
      this.aborted?.[this.aborted.length - 1]?.id?.pattern?.pattern ||
      [];
    if (patternLimit && patterns.length > patternLimit) {
      return `${patterns.slice(0, patternLimit).join(", ")} and ${patterns.length - 3} more`;
    }
    return patterns.join(", ");
  }

  getStartTimeDate(): Date {
    return timestampToDateWithFallback(this.started?.startTime, this.started?.startTimeMillis ?? 0);
  }

  getEndTimeDate(): Date {
    let durationMillis = this.getDurationMicros() / 1000;
    return new Date(this.getStartTimeDate().getTime() + durationMillis);
  }

  getFormattedStartedDate() {
    return formatDate(this.getStartTimeDate());
  }

  timeSinceStart() {
    return moment(this.getStartTimeDate()).fromNow(true);
  }

  getDurationMicros() {
    if (!this.finished && this.started) {
      return Math.max(0, new Date().getTime() - this.getStartTimeDate().getTime()) * 1000;
    }
    if (this.toolLogMap.has("elapsed time")) {
      return +this.toolLogMap.get("elapsed time")! * 1000000;
    }
    return +(this.invocation.durationUsec ?? 0);
  }

  getDurationSeconds() {
    let durationMicros = this.getDurationMicros();
    return `${durationMicros / 1000000} seconds`;
  }

  getHumanReadableDuration() {
    let durationMicros = this.getDurationMicros();
    return format.durationUsec(durationMicros);
  }

  getTiming() {
    let invocationStatus = this.invocation.invocationStatus;
    if (invocationStatus == invocation_status.InvocationStatus.DISCONNECTED_INVOCATION_STATUS) {
      return "disconnected";
    }
    if (!this.finished && this.started) {
      return this.timeSinceStart();
    }
    return this.getHumanReadableDuration();
  }

  getStatus() {
    switch (this.invocation.invocationStatus) {
      case InvocationStatus.COMPLETE_INVOCATION_STATUS:
        return this.invocation.success ? "Succeeded" : exitCode(this.invocation.bazelExitCode);
      case InvocationStatus.PARTIAL_INVOCATION_STATUS:
        return "In progress...";
      case InvocationStatus.DISCONNECTED_INVOCATION_STATUS:
        return "Disconnected";
      default:
        return "";
    }
  }

  getStatusClass() {
    switch (this.invocation.invocationStatus) {
      case InvocationStatus.COMPLETE_INVOCATION_STATUS:
        if (this.invocation.bazelExitCode == "NO_TESTS_FOUND") {
          return "neutral";
        }
        return this.invocation.success ? "success" : "failure";
      case InvocationStatus.PARTIAL_INVOCATION_STATUS:
        return "in-progress";
      case InvocationStatus.DISCONNECTED_INVOCATION_STATUS:
        return "disconnected";
      default:
        return "";
    }
  }

  isComplete() {
    return this.invocation.invocationStatus === InvocationStatus.COMPLETE_INVOCATION_STATUS;
  }

  isInProgress() {
    return this.invocation.invocationStatus === InvocationStatus.PARTIAL_INVOCATION_STATUS;
  }

  /**
   * Returns whether basic invocation metadata has been received yet, such as
   * user, bazel command, target pattern etc.
   */
  isMetadataLoaded() {
    // Just return whether we have any events. This logic works because the
    // server doesn't persist any events until all invocation metadata is
    // loaded.
    return this.invocation.event.length > 0;
  }

  getFaviconType() {
    let invocationStatus = this.invocation.invocationStatus;
    if (invocationStatus == invocation_status.InvocationStatus.DISCONNECTED_INVOCATION_STATUS) {
      return IconType.Unknown;
    }
    if (!this.started) {
      return IconType.Unknown;
    }
    if (!this.finished) {
      return IconType.InProgress;
    }
    if (this.invocation.bazelExitCode == "NO_TESTS_FOUND") {
      return IconType.Default;
    }
    return this.finished.exitCode?.code == 0 ? IconType.Success : IconType.Failure;
  }

  getStatusIcon() {
    let invocationStatus = this.invocation.invocationStatus;
    if (invocationStatus == invocation_status.InvocationStatus.DISCONNECTED_INVOCATION_STATUS) {
      return <HelpCircle className="icon" />;
    }
    if (!this.started) {
      return <HelpCircle className="icon" />;
    }
    if (!this.finished) {
      return <PlayCircle className="icon blue" />;
    }
    if (this.invocation.bazelExitCode == "NO_TESTS_FOUND") {
      return <Circle className="icon" />;
    }
    return this.finished.exitCode?.code == 0 ? (
      <CheckCircle className="icon green" />
    ) : (
      <XCircle className="icon red" />
    );
  }

  getCPU() {
    if (this.workflowConfigured) {
      // Rename GOOS/GOARCH convention to be consistent with bazel's TARGET_CPU convention.
      if (this.workflowConfigured.os === "linux" && this.workflowConfigured.arch === "amd64") {
        return "k8";
      }
      if (this.workflowConfigured.os === "darwin" && this.workflowConfigured.arch === "amd64") {
        return "darwin_x86_64";
      }
      if (this.workflowConfigured.os === "darwin" && this.workflowConfigured.arch === "arm64") {
        return "darwin_arm64";
      }
    }
    return this.configuration?.makeVariable.TARGET_CPU || "Unknown CPU";
  }

  getMode() {
    return this.configuration?.makeVariable.COMPILATION_MODE || "Unknown mode";
  }

  getDuration(completionTime: any, beginTime: any) {
    if (!completionTime || !beginTime) return "0";
    let nanos = (+completionTime.nanos - +beginTime.nanos) / 1000000000;
    return `${(+completionTime.seconds - +beginTime.seconds + nanos).toFixed(3)}`;
  }

  getRuntime(label: string) {
    let testResult = this.testSummaryMap.get(label)?.buildEvent?.testSummary;
    if (testResult) {
      let durationMillis = durationToMillisWithFallback(testResult.totalRunDuration, testResult.totalRunDurationMillis);
      return (durationMillis / 1000).toFixed(3) + " seconds";
    }
    return (
      this.getDuration(this.completedMap.get(label)?.eventTime, this.configuredMap.get(label)?.eventTime) + " seconds"
    );
  }

  getTestSize(testSize?: build_event_stream.TestSize) {
    switch (testSize) {
      case build_event_stream.TestSize.SMALL:
        return " - Small";
      case build_event_stream.TestSize.MEDIUM:
        return " - Medium";
      case build_event_stream.TestSize.LARGE:
        return " - Large";
      case build_event_stream.TestSize.ENORMOUS:
        return " - Enormous";
    }
    return "";
  }

  getLinks(): { linkUrl: string; linkText: string }[] {
    const rawValue = this.buildMetadataMap.get("BUILDBUDDY_LINKS") ?? this.workspaceStatusMap.get("BUILDBUDDY_LINKS");
    const links = rawValue?.split(",");
    const filtered: { linkUrl: string; linkText: string }[] =
      links
        ?.flatMap((link) => {
          const groups = link.match(/\[(?<linkText>.*)\]\((?<linkUrl>.*)\)/)?.groups;
          if (groups?.linkUrl) {
            return { linkUrl: groups.linkUrl, linkText: groups.linkText };
          } else {
            return [];
          }
        })
        .filter((link) => link.linkUrl.startsWith("http://") || link.linkUrl.startsWith("https://")) || [];
    return filtered;
  }

  getFiles(event?: build_event_stream.BuildEvent): build_event_stream.File[] {
    if (!event?.completed) {
      return [];
    }
    return (
      event.completed.outputGroup
        ?.flatMap((group) => group.fileSets)
        .flatMap((set) => this.fileSetIDToFilesMap.get(set.id) || []) || []
    );
  }

  isQuery() {
    return this.getCommand() == "query";
  }

  hasChunkedEventLogs(): boolean {
    return this.invocation.hasChunkedEventLogs || false;
  }

  fetchSuggestions(service: string) {
    let req = new suggestion.GetSuggestionRequest();
    if (service == "openai") {
      req.service = suggestion.SuggestionService.OPENAI;
    }
    req.invocationId = this.getInvocationId();
    return rpcService.service
      .getSuggestion(req)
      .then((res) => {
        this.botSuggestions = res.suggestion;
        this.onChange.next();
      })
      .catch((err) => {
        console.error(err);
        this.botSuggestions = ["Error getting a fix suggestion :("];
        this.onChange.next();
      });
  }

  explicitCommandLine() {
    // We allow overriding EXPLICIT_COMMAND_LINE to enable tools that wrap bazel
    // to append bazel args but still preserve the appearance of the original
    // command line. The effective command line can still be used to see the
    // effective configuration used by bazel.
    const overrideJSON = this.buildMetadataMap.get("EXPLICIT_COMMAND_LINE");
    if (overrideJSON) {
      try {
        return this.quote(JSON.parse(overrideJSON));
      } catch (_) {
        // Invalid JSON; fall back to showing BES event.
      }
    }

    return this.bazelCommandAndPatternWithOptions(this.optionsParsed?.explicitCmdLine ?? []);
  }

  bazelCommandAndPatternWithOptions(options: string[]) {
    let patterns: string[] = [];
    if (!this.hasPatternFile()) {
      patterns = this.expanded?.id?.pattern?.pattern || [];
    }
    return this.quote(["bazel", this.started?.command ?? "", ...patterns, ...(options || [])].filter((value) => value));
  }

  hasPatternFile() {
    return Boolean(this.optionsMap.get("target_pattern_file"));
  }

  // Wraps arguments containing spaces in the provided command-line in
  // quotation marks so they work when copied and pasted. The input
  // command-line is passed in as an array with one entry per piece. For
  // example, this command:
  //   "bazel build --output_filter='argument with spaces' //..."
  // is passed into this function as:
  //   ["bazel", "build", "--output_filter=argument with spaces"," "//..."],
  // and this will be returned:
  //   ["bazel", "build", "--output_filter='argument with spaces'"," "//..."],
  private quote(pieces: string[]) {
    return pieces
      .map((value) => {
        if (value.includes("=")) {
          // shlex.quote everything after the first '=' so that arguments like:
          // --flag="  = = = '' \"" are properly quoted.
          let parts: string[] = value.split("=");
          return parts[0] + "=" + shlex.quote(parts.slice(1).join("="));
        }
        return value;
      })
      .join(" ");
  }
}
