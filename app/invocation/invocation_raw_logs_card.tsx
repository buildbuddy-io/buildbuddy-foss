import { Download, PauseCircle } from "lucide-react";
import React from "react";
import { invocation } from "../../proto/invocation_ts_proto";
import Banner from "../components/banner/banner";
import LinkButton from "../components/button/link_button";
import { FilterInput } from "../components/filter_input/filter_input";
import format from "../format/format";
import rpcService from "../service/rpc_service";
import InvocationModel from "./invocation_model";

interface Props {
  model: InvocationModel;
  pageSize: number;
}

interface State {
  expandedMap: Map<Long, boolean>;
  numPages: number;
  filterString: string;
}

export default class RawLogsCardComponent extends React.Component<Props, State> {
  state: State = {
    expandedMap: new Map<Long, boolean>(),
    numPages: 1,
    filterString: "",
  };

  handleEventClicked(event: invocation.InvocationEvent) {
    this.state.expandedMap.set(event.sequenceNumber, !this.state.expandedMap.get(event.sequenceNumber));
    this.setState(this.state);
  }

  handleMoreClicked() {
    this.setState({ numPages: this.state.numPages + 1 });
  }

  handleAllClicked() {
    this.setState({ numPages: Number.MAX_SAFE_INTEGER });
  }

  handleFilterChange(event: any) {
    this.setState({ filterString: event.target.value });
  }

  render() {
    let filteredEvents = this.props.model.invocation.event
      .map((event) => {
        let json = JSON.stringify((event.buildEvent as any).toJSON(), null, 4);
        return {
          event: event,
          json: json,
        };
      })
      .filter((event) =>
        this.state.filterString ? event.json.toLowerCase().includes(this.state.filterString.toLowerCase()) : true
      );
    return (
      <>
        <FilterInput
          className="raw-logs-filter"
          value={this.state.filterString || ""}
          placeholder="Filter..."
          onChange={this.handleFilterChange.bind(this)}
        />
        <div className="card invocation-raw-logs-card">
          <PauseCircle className="icon rotate-90" />
          <div className="content">
            <div className="title">Raw logs</div>
            <LinkButton
              className="download-raw-logs-button"
              href={rpcService.getDownloadUrl({
                invocation_id: this.props.model.getInvocationId(),
                artifact: "raw_json",
              })}
              target="_blank">
              <span>Download JSON</span>
              <Download className="icon white" />
            </LinkButton>
            <div className="details code">
              <div>
                {filteredEvents
                  .slice(0, (this.props.pageSize && this.state.numPages * this.props.pageSize) || undefined)
                  .map((event) => {
                    var expanded = this.state.expandedMap.get(event.event.sequenceNumber) || this.state.filterString;
                    return (
                      <div className="raw-event">
                        <div className="raw-event-title" onClick={this.handleEventClicked.bind(this, event.event)}>
                          [{expanded ? "-" : "+"}] Build event {format.formatWithCommas(event.event.sequenceNumber)} -{" "}
                          {Object.keys(event.event.buildEvent ?? {})
                            .filter((key) => key != "id" && key != "children")
                            .join(", ")}
                        </div>
                        {expanded && <div>{event.json}</div>}
                      </div>
                    );
                  })}
              </div>
            </div>
            {this.props.pageSize &&
              filteredEvents.length > this.props.pageSize * this.state.numPages &&
              !!this.state.numPages && (
                <>
                  <div className="more" onClick={this.handleMoreClicked.bind(this)}>
                    See more events
                  </div>
                  <div className="more" onClick={this.handleAllClicked.bind(this)}>
                    See all events
                  </div>
                </>
              )}
            {this.props.model.invocation.targetGroups.length && (
              <p>
                <Banner type="info">
                  Some target-level events as well as progress events may be omitted. Click <b>Download JSON</b> to
                  download all events (some redundant event details may be truncated to reduce payload size).
                </Banner>
              </p>
            )}
          </div>
        </div>
      </>
    );
  }
}
