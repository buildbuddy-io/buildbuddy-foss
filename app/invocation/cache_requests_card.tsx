import {
  ArrowDown,
  ArrowLeftRight,
  ArrowUp,
  Check,
  ChevronRight,
  DownloadIcon,
  HelpCircle,
  ShieldClose,
  SortAsc,
  SortDesc,
  X,
} from "lucide-react";
import React from "react";
import { build_event_stream } from "../../proto/build_event_stream_ts_proto";
import { cache } from "../../proto/cache_ts_proto";
import { google as google_field_mask } from "../../proto/field_mask_ts_proto";
import { invocation_status } from "../../proto/invocation_status_ts_proto";
import { invocation } from "../../proto/invocation_ts_proto";
import { resource } from "../../proto/resource_ts_proto";
import capabilities from "../capabilities/capabilities";
import Button, { FilledButton, OutlinedButton } from "../components/button/button";
import DigestComponent from "../components/digest/digest";
import { FilterInput } from "../components/filter_input/filter_input";
import TextInput from "../components/input/input";
import { TextLink } from "../components/link/link";
import Popup from "../components/popup/popup";
import Select, { Option } from "../components/select/select";
import Spinner from "../components/spinner/spinner";
import { pinBottomMiddleToMouse, Tooltip } from "../components/tooltip/tooltip";
import error_service from "../errors/error_service";
import * as format from "../format/format";
import router from "../router/router";
import rpcService from "../service/rpc_service";
import { BuildBuddyError } from "../util/errors";
import * as proto from "../util/proto";
import { durationToMillis, timestampToDate } from "../util/proto";
import { commandWithRemoteRunnerFlags, supportsRemoteRun, triggerRemoteRun } from "../util/remote_runner";
import { subtractTimestamp } from "./invocation_execution_util";
import InvocationModel from "./invocation_model";
import LinkGithubRepoModal from "./link_github_repo_modal";

export interface CacheRequestsCardProps {
  model: InvocationModel;
  search: URLSearchParams;
  query?: string;
  show?: number;
  groupBy?: number;
  exactMatch?: boolean;
}

interface State {
  searchText: string;

  loading: boolean;
  results: cache.ScoreCard.Result[];
  nextPageToken: string;
  didInitialFetch: boolean;
  isLinkRepoModalOpen: boolean;
  showDebugCacheMissDropdown: boolean;

  digestToCacheMetadata: Map<string, cache.GetCacheMetadataResponse | null>;

  selectedDebugCacheMissOption: string;
}

const SEARCH_DEBOUNCE_INTERVAL_MS = 300;

/** How long to wait before retrying the initial fetch. */
const RETRY_FETCH_DELAY_MS = 1000;

/**
 * Represents a labeled collection of cache request filters.
 */
type PresetFilter = {
  label: string;
  values: {
    cache: resource.CacheType;
    request: cache.RequestType;
    response: cache.ResponseType;
  };
};

// Preset filters presented to the user, to avoid presenting too many query options as separate dropdowns.
const filters: PresetFilter[] = [
  {
    label: "All",
    values: { cache: 0, request: 0, response: 0 },
  },
  {
    label: "AC Hits",
    values: { cache: resource.CacheType.AC, request: cache.RequestType.READ, response: cache.ResponseType.OK },
  },
  {
    label: "AC Misses",
    values: { cache: resource.CacheType.AC, request: cache.RequestType.READ, response: cache.ResponseType.NOT_FOUND },
  },
  {
    label: "CAS Hits",
    values: { cache: resource.CacheType.CAS, request: cache.RequestType.READ, response: cache.ResponseType.OK },
  },
  {
    label: "CAS Writes",
    values: { cache: resource.CacheType.CAS, request: cache.RequestType.WRITE, response: cache.ResponseType.OK },
  },
  {
    label: "Errors",
    values: { cache: 0, request: 0, response: cache.ResponseType.ERROR },
  },
];

const defaultFilterIndex = 0; // All

/**
 * CacheRequestsCardComponent shows all BuildBuddy cache requests for an invocation in a tabular form.
 */
export default class CacheRequestsCardComponent extends React.Component<CacheRequestsCardProps, State> {
  state: State = {
    searchText: "",
    loading: false,
    results: [],
    nextPageToken: "",
    didInitialFetch: false,
    isLinkRepoModalOpen: false,
    showDebugCacheMissDropdown: false,
    digestToCacheMetadata: new Map<string, cache.GetCacheMetadataResponse>(),
    selectedDebugCacheMissOption: "identical",
  };

  constructor(props: CacheRequestsCardProps) {
    super(props);
    this.state.searchText = this.props.search.get("search") || this.props.query || "";
  }

  componentDidMount() {
    if (areResultsAvailable(this.props.model)) {
      this.fetchResults();
    }
  }

  componentDidUpdate(prevProps: Readonly<CacheRequestsCardProps>) {
    if (!areResultsAvailable(prevProps.model) && areResultsAvailable(this.props.model)) {
      this.fetchResults();
      return;
    }
    if (prevProps.search.toString() !== this.props.search.toString()) {
      // Re-fetch from the beginning when sorting/filtering parameters change
      this.fetchResults(/*pageToken=*/ "");
      return;
    }
  }

  private fetchResults(pageToken = this.state.nextPageToken) {
    this.setState({ loading: true, nextPageToken: pageToken });

    const filterFields: string[] = [];

    const filter = filters[this.getFilterIndex()].values;
    if (filter.cache) filterFields.push("cache_type");
    if (filter.request) filterFields.push("request_type");
    if (filter.response) filterFields.push("response_type");

    if (this.getSearch()) filterFields.push("search");

    const isInitialFetch = !this.state.didInitialFetch;

    rpcService.service
      .getCacheScoreCard(
        cache.GetCacheScoreCardRequest.create({
          invocationId: this.props.model.getInvocationId(),
          orderBy: this.getOrderBy(),
          descending: this.getDescending(),
          groupBy: this.getGroupBy(),
          filter: cache.GetCacheScoreCardRequest.Filter.create({
            mask: google_field_mask.protobuf.FieldMask.create({ paths: filterFields }),
            cacheType: filter.cache,
            requestType: filter.request,
            responseType: filter.response,
            search: this.getSearch(),
            exactMatch: Boolean(this.props.exactMatch),
          }),
          pageToken,
        })
      )
      .then((response) => {
        this.setState({
          results: [...(pageToken ? this.state.results : []), ...response.results],
          nextPageToken: response.nextPageToken,
        });
      })
      .catch((e: any) => this.handleFetchError(e, isInitialFetch))
      .finally(() => this.setState({ loading: false, didInitialFetch: true }));
  }

  private handleFetchError(e: any, isInitialFetch: boolean) {
    const error = BuildBuddyError.parse(e);

    // Retry NotFound errors on initial page load up to once, since stats may
    // still be getting finalized. If that still fails, just log to the console
    // since this is non-critical.
    if (error.code === "NotFound") {
      console.warn(e);
      if (isInitialFetch) {
        setTimeout(() => {
          this.fetchResults();
        }, RETRY_FETCH_DELAY_MS);
      }
      return;
    }

    error_service.handleError(e);
  }

  private getActionUrl(digestHash: string) {
    return `/invocation/${this.props.model.getInvocationId()}?actionDigest=${digestHash}#action`;
  }

  private hasSavingsData(result: cache.ScoreCard.Result): boolean {
    return Boolean(result.executionStartTimestamp && result.executionCompletedTimestamp);
  }

  private renderSavingsString(result: cache.ScoreCard.Result, compact: boolean = true) {
    if (!this.hasSavingsData(result)) {
      return "--";
    }
    const timestampUsec = subtractTimestamp(result.executionCompletedTimestamp!, result.executionStartTimestamp!);
    if (compact) {
      return format.compactDurationMillis(timestampUsec / 1000);
    } else {
      return format.durationUsec(timestampUsec);
    }
  }

  private renderSavingsColumn(result: cache.ScoreCard.Result) {
    return <div className="duration-column">{this.renderSavingsString(result)}</div>;
  }

  private renderWaterfallBar(
    result: cache.ScoreCard.Result,
    timelineStartTimeMillis: number,
    timelineDurationMillis: number
  ) {
    const resultStartTimeMillis = timestampToDate(result.startTime || {}).getTime();
    const resultDurationMillis = durationToMillis(result.duration || {});
    const beforeDurationMillis = resultStartTimeMillis - timelineStartTimeMillis;
    return (
      <div className="waterfall-column">
        <div className="waterfall-gridlines">
          <div />
          <div />
          <div />
          <div />
        </div>
        <div
          style={{
            marginLeft: `${(beforeDurationMillis / timelineDurationMillis) * 100}%`,
            width: `${(resultDurationMillis / timelineDurationMillis) * 100}%`,
          }}
          className="waterfall-bar"
        />
      </div>
    );
  }

  /**
   * Returns the start timestamp and duration for the waterfall chart.
   */
  private getStartTimestampAndDurationMillis(): [number, number] {
    const earliestStartTimeMillis = this.state.results
      .map((result) => timestampToDate(result.startTime || {}).getTime())
      .reduce((acc, cur) => Math.min(acc, cur), Number.MAX_SAFE_INTEGER);
    const invocationStartTimeMillis = this.props.model.getStartTimeDate().getTime();
    const startTimeMillis = Math.min(earliestStartTimeMillis, invocationStartTimeMillis);

    const latestEndTimeMillis = this.state.results
      .map((result) => timestampToDate(result.startTime || {}).getTime() + durationToMillis(result.duration || {}))
      .reduce((acc, cur) => Math.max(acc, cur), earliestStartTimeMillis);
    const invocationEndTimeMillis = this.props.model.getEndTimeDate().getTime();
    const endTimeMillis = Math.max(latestEndTimeMillis, invocationEndTimeMillis);

    return [startTimeMillis, endTimeMillis - startTimeMillis];
  }

  private getOrderBy() {
    return Number(this.props.search.get("sort")) as cache.GetCacheScoreCardRequest.OrderBy;
  }
  private getDescending() {
    return (this.props.search.get("desc") || "false") === "true";
  }
  private getFilterIndex() {
    return Number(
      this.props.search.get("filter") || (this.props.show === undefined ? defaultFilterIndex : this.props.show)
    );
  }
  private getGroupBy() {
    return Number(
      this.props.search.get("groupBy") || this.props.groupBy || cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_TARGET
    ) as cache.GetCacheScoreCardRequest.GroupBy;
  }
  private getSearch() {
    return this.props.search.get("search") || this.props.query || "";
  }

  private onChangeOrderBy(event: React.ChangeEvent<HTMLSelectElement>) {
    const value = Number(event.target.value) as cache.GetCacheScoreCardRequest.OrderBy;
    // When changing the sort order, set direction according to a more useful
    // default, to save an extra click. Asc is more useful for start time; Desc
    // is more useful for duration and size.
    const desc = value !== cache.GetCacheScoreCardRequest.OrderBy.ORDER_BY_START_TIME;
    router.setQuery({
      ...Object.fromEntries(this.props.search.entries() || []),
      sort: String(value),
      desc: String(desc),
    });
  }
  private onToggleDescending() {
    router.setQueryParam("desc", !this.getDescending());
  }
  private onChangeFilter(event: React.ChangeEvent<HTMLSelectElement>) {
    router.setQueryParam("filter", event.target.value);
  }
  private onChangeGroupBy(event: React.ChangeEvent<HTMLSelectElement>) {
    router.setQueryParam("groupBy", event.target.value);
  }
  private searchTimeout = 0;
  private onChangeSearch(event: React.ChangeEvent<HTMLInputElement>) {
    const searchText = event.target.value;
    // Update the search box state immediately, but debounce the query update so
    // we don't fetch on each keystroke.
    this.setState({ searchText });
    clearTimeout(this.searchTimeout);
    this.searchTimeout = window.setTimeout(
      () => router.setQueryParam("search", searchText),
      SEARCH_DEBOUNCE_INTERVAL_MS
    );
  }

  private onClickLoadMore() {
    this.fetchResults();
  }

  private renderControls(showDebugCacheMissButton: boolean) {
    return (
      <>
        <div className="controls row">
          {/* Sorting controls */}
          <label>Sort by</label>
          <Select value={this.getOrderBy()} onChange={this.onChangeOrderBy.bind(this)}>
            <Option value={cache.GetCacheScoreCardRequest.OrderBy.ORDER_BY_START_TIME}>Start time</Option>
            <Option value={cache.GetCacheScoreCardRequest.OrderBy.ORDER_BY_DURATION}>Duration</Option>
            <Option value={cache.GetCacheScoreCardRequest.OrderBy.ORDER_BY_SIZE}>Size</Option>
            <Option value={cache.GetCacheScoreCardRequest.OrderBy.ORDER_BY_CPU_SAVINGS}>CPU Savings</Option>
          </Select>
          <OutlinedButton className="icon-button" onClick={this.onToggleDescending.bind(this)}>
            {this.getDescending() ? <SortDesc className="icon" /> : <SortAsc className="icon" />}
          </OutlinedButton>
          {/* Filtering controls */}
          <div className="separator" />
          <label>Show</label>
          <Select
            debug-id="filter-cache-requests"
            value={this.getFilterIndex()}
            onChange={this.onChangeFilter.bind(this)}>
            {filters.map((filter, i) => (
              <Option key={filter.label} value={i}>
                {filter.label}
              </Option>
            ))}
          </Select>
          {/* Grouping controls */}
          <div className="separator" />
          <label>Group by</label>
          <Select value={this.getGroupBy()} onChange={this.onChangeGroupBy.bind(this)}>
            <Option value={0}>(None)</Option>
            <Option value={cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_TARGET}>Target</Option>
            <Option value={cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_ACTION}>Action</Option>
          </Select>
          {/* Debug cache miss button */}
          {capabilities.config.bazelButtonsEnabled && showDebugCacheMissButton && (
            <>
              <div className="separator" />
              <div className="debug-cache-miss-container">
                <OutlinedButton onClick={() => this.setState({ showDebugCacheMissDropdown: true })}>
                  <ShieldClose color="darkorange" />
                  <div className="debug-cache-miss-button">Debug cache misses</div>
                </OutlinedButton>
                <Popup
                  className="cache-miss-popup"
                  isOpen={this.state.showDebugCacheMissDropdown}
                  onRequestClose={() => this.setState({ showDebugCacheMissDropdown: false })}>
                  <label
                    className="checkbox-row"
                    onClick={(e) => {
                      e.stopPropagation();
                      this.setState({ selectedDebugCacheMissOption: "identical" });
                    }}>
                    <input type="radio" checked={this.state.selectedDebugCacheMissOption === "identical"} />
                    <div className="title">Between identical runs of this build</div>
                  </label>
                  <label
                    className="checkbox-row"
                    onClick={(e) => {
                      e.stopPropagation();
                      this.setState({ selectedDebugCacheMissOption: "compare" });
                    }}>
                    <input type="radio" checked={this.state.selectedDebugCacheMissOption === "compare"} />
                    <div className="title">Between invocation</div>
                  </label>
                  <div className="checkbox-row">
                    <TextInput placeholder="Invocation ID" id="debug-cache-miss-invocation-input" />
                  </div>
                  <div className="checkbox-row">
                    <FilledButton onClick={this.runBbExplain.bind(this)}>Run</FilledButton>
                  </div>
                </Popup>
              </div>
            </>
          )}
        </div>
        {!this.props.query && (
          <div className="controls row">
            <FilterInput value={this.state.searchText} onChange={this.onChangeSearch.bind(this)} />
          </div>
        )}
      </>
    );
  }

  private handleDownloadClicked(result: cache.ScoreCard.Result) {
    if (result.digest?.hash) {
      rpcService.downloadBytestreamFile(
        result.digest.hash,
        this.props.model.getBytestreamURL(result.digest),
        this.props.model.getInvocationId()
      );
    }
  }

  private renderResults(
    results: cache.ScoreCard.Result[],
    startTimeMillis: number,
    durationMillis: number,
    groupTarget: string | null = null,
    groupActionId: string | null = null
  ) {
    return results.map((result) => (
      <Tooltip
        className="row result-row"
        pin={pinBottomMiddleToMouse}
        renderContent={() => this.renderResultHovercard(result, startTimeMillis)}>
        {(groupTarget === null || groupActionId === null) && (
          <div className="name-column" title={result.targetId ? `${result.targetId} › ${result.actionMnemonic}` : ""}>
            {(() => {
              let name = "";
              /*
                 If the action ID looks like a digest, it's clearly attributed to an action.
                 If it is a special prefetcher action ID, it refers to a local action that
                 triggered the download of its input files ("input") or an action whose outputs
                 were explicitly requested ("output"). Older versions of Bazel used "prefetcher"
                 in both cases.
                 https://github.com/bazelbuild/bazel/blob/998e7624093422bee06e65965f8a575d05d57c27/src/main/java/com/google/devtools/build/lib/remote/RemoteActionInputFetcher.java#L91-L99
                 https://github.com/bazelbuild/bazel/blob/13a1ceccd9672fc9d55c716aae6e5119891e4b9b/src/main/java/com/google/devtools/build/lib/remote/RemoteActionInputFetcher.java#L91
                 In all other cases, this is a special cache access (e.g. for BES purposes) with
                 no link to an action.
                */
              if (
                looksLikeDigest(result.actionId) ||
                result.actionId === "input" ||
                result.actionId === "output" ||
                result.actionId === "prefetcher"
              ) {
                if (groupTarget === null) {
                  name = result.targetId;
                  if (groupActionId === null) {
                    name += " › ";
                  }
                }
                if (groupActionId === null) {
                  name += result.actionMnemonic;
                }
                if (result.actionId === "input") {
                  name += " (local)";
                } else if (result.actionId === "output") {
                  name += " (requested)";
                }
              } else {
                name = result.name ? result.name : result.actionId;
              }
              return looksLikeDigest(result.actionId) ? (
                <TextLink className="name-content" href={this.getActionUrl(result.actionId)}>
                  {name}
                </TextLink>
              ) : (
                <span className="name-content">{name}</span>
              );
            })()}
            <div title="Download">
              <DownloadIcon
                onClick={this.handleDownloadClicked.bind(this, result)}
                className="download-button icon"
                role="button"
              />
            </div>
          </div>
        )}
        <div className="cache-type-column" title={cacheTypeTitle(result.cacheType)}>
          {renderCacheType(result.cacheType)}
        </div>
        <div className="status-column column-with-icon">{renderStatus(result)}</div>
        {result.digest && (
          <div>
            <DigestComponent hashWidth="96px" sizeWidth="72px" digest={result.digest} expandOnHover={false} />
          </div>
        )}
        {this.isCompressedSizeColumnVisible() && (
          <div className={`compressed-size-column ${!result.compressor ? "uncompressed" : ""}`}>
            {(console.log(result), result.compressor) ? (
              <>
                <span>{renderCompressionSavings(result)}</span>
              </>
            ) : (
              <span>&nbsp;</span>
            )}
          </div>
        )}
        <div className="duration-column">
          {format.compactDurationMillis(proto.durationToMillis(result.duration ?? {}))}
        </div>
        {capabilities.config.trendsSummaryEnabled && this.renderSavingsColumn(result)}
        {this.renderWaterfallBar(result, startTimeMillis, durationMillis)}
      </Tooltip>
    ));
  }

  private getCacheMetadata(scorecardResult: cache.ScoreCard.Result) {
    const digest = scorecardResult.digest;
    if (!digest?.hash) {
      return;
    }
    const remoteInstanceName = this.props.model.getRemoteInstanceName();

    // Set an empty struct in the map so the FE doesn't fire duplicate requests while the first request is in progress
    // or if there is an invalid result
    this.state.digestToCacheMetadata.set(digest.hash, null);

    const service = rpcService.getRegionalServiceOrDefault(this.props.model.stringCommandLineOption("remote_cache"));

    service
      .getCacheMetadata(
        cache.GetCacheMetadataRequest.create({
          resourceName: resource.ResourceName.create({
            digest: digest,
            cacheType: scorecardResult.cacheType,
            instanceName: remoteInstanceName,
          }),
        })
      )
      .then((response) => {
        this.state.digestToCacheMetadata.set(digest.hash, response);
        this.forceUpdate();
      })
      .catch((e) => {
        console.log("Could not fetch metadata: " + BuildBuddyError.parse(e));
      });
  }

  private renderResultHovercard(result: cache.ScoreCard.Result, startTimeMillis: number) {
    if (!result.digest?.hash) {
      return null;
    }
    const digest = result.digest.hash;
    // TODO(bduffany): Add an `onHover` prop to <Tooltip> and move this logic there
    if (!this.state.digestToCacheMetadata.has(digest)) {
      this.getCacheMetadata(result);
    }

    const cacheMetadata = this.state.digestToCacheMetadata.get(digest);

    let lastAccessed = "";
    let lastModified = "";
    if (cacheMetadata) {
      if (cacheMetadata.lastAccessUsec) {
        const lastAccessedMs = Number(cacheMetadata.lastAccessUsec) / 1000;
        lastAccessed = format.formatDate(new Date(lastAccessedMs));
      }

      if (cacheMetadata.lastModifyUsec) {
        const lastModifiedMs = Number(cacheMetadata.lastModifyUsec) / 1000;
        lastModified = format.formatDate(new Date(lastModifiedMs));
      }
    }

    return (
      <div className="cache-result-hovercard">
        {result.targetId ? (
          <>
            <b>Target</b> <span>{result.targetId}</span>
          </>
        ) : null}
        {result.actionMnemonic && (
          <>
            <b>Action mnemonic</b>
            <span>{result.actionMnemonic}</span>
          </>
        )}
        {result.actionId && (
          <>
            <b>Action ID</b>
            <span>
              {result.actionId === "input"
                ? "input (to local execution of this action)"
                : result.actionId === "output"
                  ? "output (explicitly requested)"
                  : result.actionId}{" "}
            </span>
          </>
        )}
        {result.name ? (
          <>
            <b>File</b>{" "}
            <span>
              {result.pathPrefix ? result.pathPrefix + "/" : ""}
              {result.name}
            </span>
          </>
        ) : null}
        <>
          <b>Size</b>
          <span>{format.bytes(result.digest?.sizeBytes ?? 0)}</span>
        </>
        {result.compressor ? (
          <>
            <b>Compressed</b>{" "}
            <span>
              {format.bytes(result.transferredSizeBytes)} {renderCompressionSavings(result)}
            </span>
          </>
        ) : null}
        <>
          {/* Timestamp relative to the invocation start time */}
          <b>Started at</b>{" "}
          <span>
            {format.durationMillis(proto.timestampToDate(result.startTime ?? {}).getTime() - startTimeMillis)}
          </span>
        </>
        <>
          <b>Duration</b> <span>{format.durationMillis(proto.durationToMillis(result.duration ?? {}))}</span>
          {capabilities.config.trendsSummaryEnabled && this.hasSavingsData(result) && (
            <>
              <b>CPU saved by cache hit</b>
              <span>{this.renderSavingsString(result, false)}</span>
            </>
          )}
        </>
        {lastAccessed ? (
          <>
            <b>Last accessed</b> <span>{lastAccessed}</span>
          </>
        ) : null}
        {lastModified ? (
          <>
            <b>Last modified</b> <span>{lastModified}</span>
          </>
        ) : null}
      </div>
    );
  }

  private isCompressedSizeColumnVisible() {
    return (
      this.props.model.isCacheCompressionEnabled() &&
      filters[this.getFilterIndex()].values.cache !== resource.CacheType.AC
    );
  }

  private durationTooltip() {
    return (
      <Tooltip
        renderContent={() => (
          <div className="cache-requests-card-hovercard">
            <div>
              <p>
                <b>Duration</b>
              </p>
              <p>
                The time difference between when the server started processing the request and finished sending all
                requested bytes.
              </p>
              <p>This time is measured by the server, and may not include load balancer or client processing time.</p>
            </div>
          </div>
        )}>
        <HelpCircle className="icon" />
      </Tooltip>
    );
  }

  private savingsTooltip() {
    return (
      <Tooltip
        renderContent={() => (
          <div className="cache-requests-card-hovercard">
            <div>
              <p>
                <b>Savings</b>
              </p>
              <p>Action Cache (AC) items only. The amount of wall time that was saved by using this cached action.</p>
              <p>
                Determined by looking at how long the action took to build when it was originally written to the cache.
              </p>
            </div>
          </div>
        )}>
        <HelpCircle className="icon" />
      </Tooltip>
    );
  }

  private waterfallTooltip() {
    return (
      <Tooltip
        renderContent={() => (
          <div className="cache-requests-card-hovercard">
            <div>
              <p>
                <b>Waterfall</b>
              </p>
              <p>When this cache request took place in the overall timeline of the build.</p>
            </div>
          </div>
        )}>
        <HelpCircle className="icon" />
      </Tooltip>
    );
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

  private async runBbExplain() {
    const repoURL = this.props.model.getRepo();
    if (repoURL.length == 0) {
      alert("Repo URL required.");
      return;
    }

    const isSupported = await supportsRemoteRun(repoURL);
    if (!isSupported) {
      this.setState({ isLinkRepoModalOpen: true });
      return;
    }

    const currentCommand = this.props.model.explicitCommandLine();
    let generateExecLogCmd1: string;
    let execLogOrInvocationId1: string;
    if (CacheRequestsCardComponent.hasExecLog(this.props.model)) {
      execLogOrInvocationId1 = this.props.model.getInvocationId();
      generateExecLogCmd1 = "";
    } else {
      execLogOrInvocationId1 = "inv1.log";
      generateExecLogCmd1 = commandWithRemoteRunnerFlags(
        currentCommand + " --experimental_execution_log_compact_file=" + execLogOrInvocationId1
      );
    }

    let generateExecLogCmd2: string;
    let execLogOrInvocationId2: string;
    if (this.state.selectedDebugCacheMissOption == "compare") {
      const compareInvocationId = (document.getElementById("debug-cache-miss-invocation-input") as HTMLInputElement)
        .value;
      if (!compareInvocationId) {
        alert("Invocation ID is required.");
        return;
      }
      const compareInv = await this.fetchInvocation(compareInvocationId);
      const compareModel = new InvocationModel(compareInv);

      if (CacheRequestsCardComponent.hasExecLog(compareModel)) {
        generateExecLogCmd2 = "";
        execLogOrInvocationId2 = compareModel.getInvocationId();
      } else {
        if (compareModel.getRepo().length == 0) {
          alert("Repo URL for comparison invocation required.");
          return;
        }
        if (repoURL != compareModel.getRepo()) {
          alert("The GitHub repo of the comparison invocation must match the current invocation's repo.");
          return;
        }

        const compareCommit = compareModel.getCommit();
        execLogOrInvocationId2 = "inv2.log";
        generateExecLogCmd2 = `
git fetch origin ${compareCommit}
git checkout ${compareCommit}
${commandWithRemoteRunnerFlags(compareModel.explicitCommandLine() + " --experimental_execution_log_compact_file=" + execLogOrInvocationId2)}`;
      }
    } else {
      // Force a rerun of the identical invocation to detect non-reproducibility.
      execLogOrInvocationId2 = "inv2.log";
      generateExecLogCmd2 = commandWithRemoteRunnerFlags(
        currentCommand + " --experimental_execution_log_compact_file=" + execLogOrInvocationId2
      );
    }

    const command = `
curl -fsSL https://install.buildbuddy.io | bash
${generateExecLogCmd1}
${generateExecLogCmd2}
output=$(bb explain --old ${execLogOrInvocationId1} --new ${execLogOrInvocationId2})
if [ -z "$output" ]; then
    echo "There are no differences between the compact execution logs of the two invocations."
else
  printf "%s\\n" "$output"
fi
`;
    let platformProps = new Map([["EstimatedComputeUnits", "3"]]);
    triggerRemoteRun(this.props.model, command, false /*autoOpenChild*/, platformProps);
    this.setState({ showDebugCacheMissDropdown: false });
  }

  private static hasExecLog(invocation: InvocationModel): boolean {
    return Boolean(
      invocation.buildToolLogs?.log.some(
        (log: build_event_stream.File) =>
          log.name == "execution_log.binpb.zst" && log.uri && Boolean(log.uri.startsWith("bytestream://"))
      )
    );
  }

  private async fetchInvocation(invocationId: string): Promise<invocation.Invocation> {
    const response = await rpcService.service.getInvocation(
      new invocation.GetInvocationRequest({
        lookup: new invocation.InvocationLookup({
          invocationId,
        }),
      })
    );
    return response.invocation[0];
  }

  render() {
    if (this.state.loading && !this.state.results.length) {
      return (
        <RequestsCardContainer>
          {this.renderControls(false /*showDebugCacheMissButton*/)}
          <div className="loading" />
        </RequestsCardContainer>
      );
    }

    if (!areResultsAvailable(this.props.model)) {
      return (
        <RequestsCardContainer>
          <div>Cache requests will be shown here when the invocation has completed.</div>
        </RequestsCardContainer>
      );
    }

    if (!this.state.results.length) {
      return (
        <RequestsCardContainer>
          {this.renderControls(false /*showDebugCacheMissButton*/)}
          <div>No cache requests found.</div>
        </RequestsCardContainer>
      );
    }

    const groups = this.getGroupBy() ? groupResults(this.state.results, this.getGroupBy()) : null;
    const [startTimeMillis, durationMillis] = this.getStartTimestampAndDurationMillis();

    return (
      <RequestsCardContainer
        className={
          this.getGroupBy() === cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_TARGET ? "group-by-target" : ""
        }>
        {this.renderControls(true /*showDebugCacheMissButton*/)}
        <LinkGithubRepoModal
          isOpen={this.state.isLinkRepoModalOpen}
          onRequestClose={() => this.setState({ isLinkRepoModalOpen: false })}
        />
        <div debug-id="cache-results-table" className="results-table">
          {/* When the results are being replaced, show an overlay indicating that the
              current results are stale. */}
          {this.state.loading && !this.state.nextPageToken && (
            <>
              <div className="loading-overlay" />
              <div className="loading loading-slim results-updating" />
            </>
          )}

          <div className="row column-headers">
            {this.getGroupBy() !== cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_ACTION && (
              <div className="name-column">Name</div>
            )}
            <div className="cache-type-column">Cache</div>
            <div className="status-column">Status</div>
            <div className="digest-column">Digest (hash/size)</div>
            {this.isCompressedSizeColumnVisible() && <div className="compressed-size-column">Compression</div>}
            <div className="duration-column">Duration {this.durationTooltip()}</div>
            {capabilities.config.trendsSummaryEnabled && (
              <div className="duration-column">Savings {this.savingsTooltip()}</div>
            )}
            <div className="waterfall-column">Waterfall {this.waterfallTooltip()}</div>
          </div>
          {groups === null && (
            <div className="results-list column">
              {this.renderResults(this.state.results, startTimeMillis, durationMillis)}
            </div>
          )}
          {groups?.map((group) => (
            <div className="group">
              <div className="group-title action-id row">
                <div className="row action-label">
                  {/* Always render the target label when grouping by target or action. */}
                  <div>{group.results[0]?.targetId || "(Unknown target)"}</div>
                  {/* Then if grouping by action, show a chevron (">") followed by the action name. */}
                  {this.getGroupBy() === cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_ACTION && (
                    <>
                      <ChevronRight className="icon chevron" />
                      {/* If we have an action ID that looks like a digest, render it as a link
                          to the action page. */}
                      {group.actionId && looksLikeDigest(group.actionId) && (
                        <TextLink className="action-mnemonic" href={this.getActionUrl(group.actionId)}>
                          {group.results[0]?.actionMnemonic || `(Unknown action ${group.actionId})`}
                        </TextLink>
                      )}
                      {/* Otherwise render the mnemonic (if available) or the plaintext action ID,
                          which will be something like "bes-upload" or "remote-download".
                        */}
                      {!looksLikeDigest(group.actionId) && (
                        <div className="action-mnemonic">
                          {group.results[0]?.actionMnemonic || group.results[0]?.actionId || "(Unknown action)"}
                        </div>
                      )}
                    </>
                  )}
                  {capabilities.config.bazelButtonsEnabled &&
                    group.results[0]?.targetId &&
                    group.results[0]?.targetId.startsWith("//") && (
                      <>
                        <Tooltip
                          className="row"
                          pin={pinBottomMiddleToMouse}
                          renderContent={() => (
                            <div className="cache-result-hovercard">
                              <div>Why did this target build?</div>
                            </div>
                          )}>
                          <HelpCircle
                            onClick={this.executeRemoteBazelQuery.bind(this, group.results[0]?.targetId!)}
                            className="download-button icon"
                            role="button"
                          />
                        </Tooltip>
                      </>
                    )}
                </div>
              </div>
              <div className="group-contents results-list column">
                {this.renderResults(group.results, startTimeMillis, durationMillis, group.targetId, group.actionId)}
              </div>
            </div>
          ))}
        </div>
        <div className="table-footer-controls">
          {this.state.nextPageToken && (
            <Button
              className="load-more-button"
              onClick={this.onClickLoadMore.bind(this)}
              disabled={this.state.loading}>
              <span>Load more</span>
              {this.state.loading && <Spinner className="white" />}
            </Button>
          )}
        </div>
      </RequestsCardContainer>
    );
  }
}

function areResultsAvailable(model: InvocationModel): boolean {
  return model.invocation.invocationStatus !== invocation_status.InvocationStatus.PARTIAL_INVOCATION_STATUS;
}

const RequestsCardContainer: React.FC<JSX.IntrinsicElements["div"]> = ({ className, children, ...props }) => (
  <div className={`card cache-requests-card ${className || ""}`} {...props}>
    <div className="content">
      <div className="title">
        <ArrowLeftRight className="icon" />
        <span>Cache requests</span>
      </div>
      <div className="details">{children}</div>
    </div>
  </div>
);

function renderCacheType(cacheType: resource.CacheType): React.ReactNode {
  switch (cacheType) {
    case resource.CacheType.CAS:
      return "CAS";
    case resource.CacheType.AC:
      return "AC";
    default:
      return "";
  }
}

function cacheTypeTitle(cacheType: resource.CacheType): string | undefined {
  switch (cacheType) {
    case resource.CacheType.CAS:
      return "Content addressable storage";
    case resource.CacheType.AC:
      return "Action cache";
    default:
      return undefined;
  }
}

function renderCompressionSavings(result: cache.ScoreCard.Result) {
  const compressionSavings = 1 - Number(result.transferredSizeBytes) / Number(result.digest?.sizeBytes ?? 1);
  return (
    <span className={`compression-savings ${compressionSavings > 0 ? "positive" : "negative"}`}>
      {compressionSavings <= 0 ? "+" : ""}
      {(-compressionSavings * 100).toPrecision(3)}%
    </span>
  );
}

function renderStatus(result: cache.ScoreCard.Result): React.ReactNode {
  if (result.requestType === cache.RequestType.READ) {
    if (result.status?.code !== 0 /*=OK*/) {
      return (
        <>
          <X className="icon red" />
          <span>Miss</span>
          {/* TODO: Show "Error" if status code is something other than NotFound */}
        </>
      );
    }
    return (
      <>
        {result.cacheType === resource.CacheType.AC ? (
          <Check className="icon green" />
        ) : (
          <ArrowDown className="icon green" />
        )}
        <span>Hit</span>
      </>
    );
  }
  if (result.requestType === cache.RequestType.WRITE) {
    if (result.status?.code !== 0 /*=OK*/) {
      return (
        <>
          <X className="icon red" />
          <span>Error</span>
        </>
      );
    }
    return (
      <>
        <ArrowUp className="icon red" />
        <span>Write</span>
      </>
    );
  }
  return "";
}

type ResultGroup = {
  // Target ID or null if not grouping by anything.
  targetId: string | null;
  // Action ID or null if not grouping by anything / grouping only by target.
  actionId: string | null;
  results: cache.ScoreCard.Result[];
};

function getGroupKey(result: cache.ScoreCard.Result, groupBy: cache.GetCacheScoreCardRequest.GroupBy): string | null {
  if (!groupBy) return null;
  if (groupBy === cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_ACTION) return result.actionId;
  return result.targetId;
}

/**
 * The server groups into contiguous runs of results with the same group key.
 * This un-flattens the runs into a list of groups.
 */
function groupResults(
  results: cache.ScoreCard.Result[],
  groupBy: cache.GetCacheScoreCardRequest.GroupBy
): ResultGroup[] {
  const out: ResultGroup[] = [];
  let curRun: ResultGroup | null = null;
  let curGroup: string | null = null;
  for (const result of results) {
    const groupKey = getGroupKey(result, groupBy);
    if (!curRun || groupKey !== curGroup) {
      curRun = {
        targetId: groupBy ? result.targetId : null,
        actionId: groupBy === cache.GetCacheScoreCardRequest.GroupBy.GROUP_BY_ACTION ? result.actionId : null,
        results: [],
      };
      curGroup = groupKey;
      out.push(curRun!);
    }
    curRun!.results.push(result);
  }
  return out;
}

/**
 * Bazel includes some action IDs like "bes-upload" so we use this logic to try
 * and tell those apart from digests.
 */
function looksLikeDigest(actionId: string | null) {
  return actionId?.length === 64;
}
