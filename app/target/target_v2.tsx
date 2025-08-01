import { Box, CheckCircle, Clock, Copy, Hash, HelpCircle, History, SkipForward, Target, XCircle } from "lucide-react";
import React from "react";
import { api as api_common } from "../../proto/api/v1/common_ts_proto";
import { build_event_stream } from "../../proto/build_event_stream_ts_proto";
import { target } from "../../proto/target_ts_proto";
import alert_service from "../alert/alert_service";
import { User } from "../auth/auth_service";
import capabilities from "../capabilities/capabilities";
import { OutlinedButton } from "../components/button/button";
import { OutlinedLinkButton } from "../components/button/link_button";
import format from "../format/format";
import CacheRequestsCardComponent from "../invocation/cache_requests_card";
import InvocationModel from "../invocation/invocation_model";
import LinkGithubRepoModal from "../invocation/link_github_repo_modal";
import { renderTestSize } from "../invocation/target_util";
import router from "../router/router";
import rpc_service from "../service/rpc_service";
import { copyToClipboard } from "../util/clipboard";
import { addDurationToTimestamp, timestampToDateWithFallback } from "../util/proto";
import { commandWithRemoteRunnerFlags, supportsRemoteRun, triggerRemoteRun } from "../util/remote_runner";
import ActionCardComponent from "./action_card";
import FlakyTargetChipComponent from "./flaky_target_chip";
import TargetArtifactsCardComponent from "./target_artifacts_card";
import TargetTestCoverageCardComponent from "./target_test_coverage_card";
import TargetTestDocumentCardComponent from "./target_test_document_card";
import TargetTestLogCardComponent from "./target_test_log_card";

const Status = api_common.v1.Status;

export interface TargetProps {
  invocationId: string;
  label: string;
  status: api_common.v1.Status;
  model: InvocationModel;
  search: URLSearchParams;

  user?: User;
  repo?: string;
  commit?: string;
  tab: string;

  dark: boolean;
}

interface State {
  loading: boolean;
  target?: target.Target;
  isLinkRepoModalOpen: boolean;
}

export default class TargetV2Component extends React.Component<TargetProps, State> {
  state: State = {
    loading: false,
    isLinkRepoModalOpen: false,
  };

  componentDidMount() {
    document.title = `Target ${this.props.label} | BuildBuddy`;
    this.fetch();
  }

  private fetch() {
    this.setState({ loading: true });
    rpc_service.service
      .getTarget({
        invocationId: this.props.invocationId,
        targetLabel: this.props.label,
        status: this.props.status,
      })
      .then((response) => this.setState({ target: response.targetGroups[0]?.targets[0] }))
      .catch(() => {
        this.setState({
          target: new target.Target(),
        });
      })
      .finally(() => this.setState({ loading: false }));

    // TODO: maybe refresh every 3s to handle the case where the invocation is
    // still in progress and NamedSetOfFiles events are still being published
    // for the target
  }

  private handleOrganizationClicked() {
    router.navigateHome();
  }

  private handleInvocationClicked() {
    router.navigateToInvocation(this.props.invocationId);
  }

  renderStatusIcon(): React.ReactNode {
    if (!this.state.target) return null;

    switch (this.state.target.status) {
      case Status.FAILED_TO_BUILD:
      case Status.FAILED:
      case Status.CANCELLED:
      case Status.INCOMPLETE:
        return <XCircle className="icon red" />;
      case Status.BUILT:
      case Status.PASSED:
        return <CheckCircle className="icon green" />;
      case Status.FLAKY:
        return <HelpCircle className="icon orange" />;
      case Status.TIMED_OUT:
        return <Clock className="icon" />;
      case Status.SKIPPED:
        return <SkipForward className="icon purple" />;
      default:
        return <HelpCircle className="icon gray" />;
    }
  }

  getTargetStatusTitle() {
    switch (this.state.target?.status) {
      case Status.BUILDING:
        return "Building";
      case Status.TESTING:
        return "Testing";
      case Status.BUILT:
        return "Built successfully";
      case Status.PASSED:
        return "Passed";
      case Status.FLAKY:
        return "Flaky";
      case Status.TIMED_OUT:
        return "Timeout";
      case Status.CANCELLED:
        return "Cancelled";
      case Status.FAILED:
        return "Failed";
      case Status.FAILED_TO_BUILD:
        return "Failed to build";
      case Status.INCOMPLETE:
        return "Incomplete";
      case Status.TOOL_FAILED:
        return "Tool failed";
      default:
        return "Unknown";
    }
  }

  getTestResultStatusClass(status: build_event_stream.TestStatus) {
    switch (status) {
      case build_event_stream.TestStatus.PASSED:
        return "test-passed";
      case build_event_stream.TestStatus.FLAKY:
        return "test-flaky";
      case build_event_stream.TestStatus.TIMEOUT:
      case build_event_stream.TestStatus.FAILED:
      case build_event_stream.TestStatus.REMOTE_FAILURE:
      case build_event_stream.TestStatus.FAILED_TO_BUILD:
        return "test-failed";
      case build_event_stream.TestStatus.INCOMPLETE:
        return "test-error";
      default:
        return "test-error";
    }
  }

  getTestSize(): string {
    if (!this.state.target || !this.state.target.metadata?.testSize) return "";
    const adjective = renderTestSize(this.state.target.metadata?.testSize);
    if (adjective === "") return "";
    return adjective + " test";
  }

  resultSort(a: build_event_stream.BuildEvent, b: build_event_stream.BuildEvent) {
    let statusDiff = (b?.testResult?.status ?? 0) - (b?.testResult?.status ?? 0);
    if (statusDiff != 0) {
      return statusDiff;
    }
    let shardDiff = (a?.id?.testResult?.shard ?? 0) - (b?.id?.testResult?.shard ?? 0);
    if (shardDiff != 0) {
      return shardDiff;
    }
    let runDiff = (a?.id?.testResult?.run ?? 0) - (b?.id?.testResult?.run ?? 0);
    if (runDiff != 0) {
      return runDiff;
    }
    return (a?.id?.testResult?.attempt ?? 0) - (b?.id?.testResult?.attempt ?? 0);
  }

  actionSort(a: build_event_stream.BuildEvent, b: build_event_stream.BuildEvent) {
    return (b?.action?.exitCode ?? 0) - (a?.action?.exitCode ?? 0);
  }

  getTime(): string {
    const testSummary = this.state.target?.testSummary;
    if (testSummary?.lastStopTime || testSummary?.lastStopTimeMillis) {
      const lastStopDate = timestampToDateWithFallback(testSummary?.lastStopTime, testSummary?.lastStopTimeMillis);
      return format.formatDate(lastStopDate);
    }
    if (!this.state.target?.timing?.startTime) return "";
    const endTime = addDurationToTimestamp(this.state.target.timing.startTime, this.state.target.timing.duration);
    return format.formatTimestamp(endTime);
  }

  handleCopyClicked(label: string) {
    copyToClipboard(label);
    alert_service.success("Label copied to clipboard!");
  }

  getTargetHistoryURL() {
    // Test history doesn't work without a repo selected.  We only show this
    // link if the target has a testSummary event because we only store target
    // history for tests.
    // TODO(https://github.com/buildbuddy-io/buildbuddy-internal/issues/3584):
    // support target history for plain-old-build targets, too.
    if (!this.props.repo || !this.state.target || !this.state.target.testSummary) return "";

    const search = new URLSearchParams({
      filter: this.props.label,
      repo: this.props.repo,
    });
    return `/tests/?${search}`;
  }

  generateRunName(testResult: build_event_stream.BuildEventId.ITestResultId) {
    return `Run ${testResult.run} (Attempt ${testResult.attempt}, Shard ${testResult.shard})`;
  }

  getTab() {
    // If the user explicitly clicked on a tab, show that
    if (this.props.tab) {
      return this.props.tab;
    }

    // If any of the attempts didn't pass, let's default ot that one.
    let events = this.state.target?.testResultEvents?.sort(this.resultSort) || [];
    for (let [i, event] of events.entries()) {
      if (event.testResult?.status != build_event_stream.TestStatus.PASSED) {
        return `#${i + 1}`;
      }
    }

    // If all attempts passed, let's fall back to the first one.
    return "#1";
  }

  async executeRemoteBazelQuery(target: string) {
    const isSupported = await supportsRemoteRun(this.props.model.getRepo());
    if (!isSupported) {
      this.setState({ isLinkRepoModalOpen: true });
      return;
    }
    const command = commandWithRemoteRunnerFlags(
      `bazel query "allpaths(${this.props.model.invocation.pattern}, ${target})" --output=graph`
    );
    triggerRemoteRun(this.props.model, command, true /*autoOpenChild*/, null);
  }

  render() {
    const historyURL = this.getTargetHistoryURL();
    const target = this.state.target;
    if (!target) {
      return (
        <div className="target-page">
          <div className="loading" />
        </div>
      );
    }
    const resultEvents = target.testResultEvents?.sort(this.resultSort) || [];
    const actionEvents = target.actionEvents?.sort(this.actionSort) || [];
    return (
      <div className="target-page">
        <div className="shelf">
          <div className="container">
            <div className="breadcrumbs">
              {this.props.user && (
                <span onClick={this.handleOrganizationClicked.bind(this)} className="clickable">
                  {this.props.user?.selectedGroupName()}
                </span>
              )}
              {this.props.user && (
                <span onClick={this.handleOrganizationClicked.bind(this)} className="clickable">
                  Builds
                </span>
              )}
              <span onClick={this.handleInvocationClicked.bind(this)} className="clickable">
                Invocation {this.props.invocationId}
              </span>
              <span>Target {this.props.label}</span>
            </div>
            <div className="titles">
              <div className="title">
                {this.props.label}{" "}
                <Copy className="copy-icon" onClick={this.handleCopyClicked.bind(this, this.props.label)} />
              </div>
              <div className="subtitle">{this.getTime()}</div>
              {historyURL && (
                <OutlinedLinkButton href={historyURL} className="target-history-button">
                  <History className="icon" />
                  <span>Target history</span>
                </OutlinedLinkButton>
              )}
              {capabilities.config.targetFlakesUiEnabled &&
                this.props.repo &&
                isPotentialFailureOrFlakeStatus(this.props.status) && (
                  <FlakyTargetChipComponent
                    labels={[this.props.label]}
                    repo={this.props.repo}></FlakyTargetChipComponent>
                )}
              {capabilities.config.bazelButtonsEnabled && (
                <>
                  <OutlinedButton className="" onClick={this.executeRemoteBazelQuery.bind(this, this.props.label)}>
                    <HelpCircle className="icon blue" />
                    <span>Why did this build?</span>
                  </OutlinedButton>
                  <LinkGithubRepoModal
                    isOpen={this.state.isLinkRepoModalOpen}
                    onRequestClose={() => this.setState({ isLinkRepoModalOpen: false })}
                  />
                </>
              )}
            </div>
            <div className="details">
              {Boolean(target?.status) && (
                <div className="detail">
                  {this.renderStatusIcon()}
                  {this.getTargetStatusTitle()}
                </div>
              )}

              {target?.testSummary && (
                <div className="detail">
                  <Hash className="icon" />
                  {target.testSummary.totalRunCount ?? 0} total runs
                </div>
              )}
              {Boolean(target?.metadata?.ruleType || target?.actionEvents.length) && (
                <div className="detail">
                  <Target className="icon" />
                  {target?.metadata?.ruleType ||
                    target?.actionEvents?.map((actionEvent) => actionEvent?.action?.type).join(",")}
                </div>
              )}
              {Boolean(target.metadata?.testSize) && (
                <div className="detail">
                  <Box className="icon" />
                  {this.getTestSize()}
                </div>
              )}
            </div>
          </div>
        </div>
        <div className="container nopadding-dense">
          {resultEvents.length > 1 && (
            <div className={`runs ${resultEvents.length > 9 && "run-grid"}`}>
              {resultEvents.map((event, index) => (
                <a
                  href={`#${index + 1}`}
                  title={this.generateRunName(event?.id?.testResult ?? {})}
                  className={`run ${this.getTestResultStatusClass(
                    event.testResult?.status ?? build_event_stream.TestStatus.NO_STATUS
                  )} ${this.getTab() == `#${index + 1}` ? "selected" : ""}`}>
                  Run {event.id?.testResult?.run ?? 0} (Attempt {event.id?.testResult?.attempt ?? 0}, Shard{" "}
                  {event.id?.testResult?.shard ?? 0}
                  {event.testResult?.cachedLocally
                    ? ", Cached locally"
                    : event.testResult?.executionInfo?.cachedRemotely
                      ? ", Cached remotely"
                      : ""}
                  )
                </a>
              ))}
            </div>
          )}
          {resultEvents
            .filter((_, index) => `#${index + 1}` == this.getTab())
            .map((buildEvent) => (
              <span>
                <TargetTestDocumentCardComponent
                  dark={this.props.dark}
                  invocationId={this.props.invocationId}
                  buildEvent={buildEvent}
                />
                <TargetTestLogCardComponent
                  dark={this.props.dark}
                  invocationId={this.props.invocationId}
                  buildEvent={buildEvent}
                />
                <TargetTestCoverageCardComponent
                  invocationId={this.props.invocationId}
                  repo={this.props.repo || ""}
                  commit={this.props.commit || ""}
                  buildEvent={buildEvent}
                />
              </span>
            ))}
          {actionEvents.map((action) => (
            <ActionCardComponent dark={this.props.dark} invocationId={this.props.invocationId} buildEvent={action} />
          ))}
          {Boolean(target.files.length) && (
            <TargetArtifactsCardComponent
              name={"Target outputs"}
              invocationId={this.props.invocationId}
              files={target.files}
            />
          )}
          {resultEvents
            .filter((event, index) => `#${index + 1}` == this.getTab() && event?.testResult?.testActionOutput)
            .map((event) => (
              <div>
                <TargetArtifactsCardComponent
                  name={this.generateRunName(event?.id?.testResult ?? {})}
                  invocationId={this.props.invocationId}
                  files={event?.testResult?.testActionOutput as build_event_stream.File[]}
                />
              </div>
            ))}

          {capabilities.config.detailedCacheStatsEnabled && (
            <CacheRequestsCardComponent
              model={this.props.model}
              query={this.props.label}
              search={this.props.search}
              groupBy={1} // Action
              show={0} // All
              exactMatch={true}
            />
          )}
        </div>
      </div>
    );
  }
}

function isPotentialFailureOrFlakeStatus(s: api_common.v1.Status): boolean {
  switch (s) {
    case Status.BUILDING:
    case Status.BUILT:
    case Status.TESTING:
    case Status.PASSED:
    case Status.SKIPPED:
      return false;
    default:
      return true;
  }
}
