import { ChevronRight, Copy } from "lucide-react";
import React from "react";
import { build_event_stream } from "../../proto/build_event_stream_ts_proto";
import alert_service from "../alert/alert_service";
import Link from "../components/link/link";
import format from "../format/format";
import { copyToClipboard } from "../util/clipboard";
import InvocationModel from "./invocation_model";

interface Props {
  model: InvocationModel;
  icon: JSX.Element;
  buildEvents: build_event_stream.BuildEvent[];
  pastVerb: string;
  presentVerb: string;
  pageSize: number;
  filter: string;
  className: string;
}

interface State {
  numPages: number;
}

export default class TargetsCardComponent extends React.Component<Props, State> {
  state: State = {
    numPages: 1,
  };

  private handleMoreClicked() {
    this.setState({ numPages: this.state.numPages + 1 });
  }

  private handleCopyClicked(label: string) {
    copyToClipboard(label);
    alert_service.success("Target labels copied to clipboard!");
  }

  render() {
    let events = this.props.buildEvents.filter(
      (target) =>
        !this.props.filter || target.id?.targetCompleted?.label.toLowerCase().includes(this.props.filter.toLowerCase())
    );
    const rootCauseEvents = events.filter((event) =>
      this.props.model.rootCauseTargetLabels.has(event.id?.targetCompleted?.label ?? "")
    );
    const otherEvents = events.filter(
      (event) => !this.props.model.rootCauseTargetLabels.has(event.id?.targetCompleted?.label ?? "")
    );
    events = [...rootCauseEvents, ...otherEvents];

    return (
      <div className={`card ${this.props.className} invocation-targets-card`}>
        <div className="icon">{this.props.icon}</div>
        <div className="content">
          <div className="title">
            {format.formatWithCommas(events.length)}
            {this.props.filter ? " matching" : ""} {this.props.pastVerb}{" "}
            <Copy
              className="copy-icon"
              onClick={this.handleCopyClicked.bind(
                this,
                events.map((target) => target.id?.targetCompleted?.label ?? "").join(" ")
              )}
            />
          </div>
          <div className="details">
            <div className="targets-table">
              {events
                .slice(0, (this.props.pageSize && this.state.numPages * this.props.pageSize) || undefined)
                .map((target) => (
                  <Link
                    className="target-row"
                    href={`?target=${encodeURIComponent(target.id?.targetCompleted?.label ?? "")}`}>
                    <div
                      title={`${
                        this.props.model.configuredMap.get(target.id?.targetCompleted?.label ?? "")?.buildEvent
                          ?.configured?.targetKind
                      } ${this.props.model.getTestSize(
                        this.props.model.configuredMap.get(target.id?.targetCompleted?.label ?? "")?.buildEvent
                          ?.configured?.testSize
                      )}`}
                      className="target">
                      <span className="target-status-icon">{this.props.icon}</span>{" "}
                      <span className="chevron-icon">
                        <ChevronRight className="icon" />
                      </span>
                      <span className="target-label">{target.id?.targetCompleted?.label}</span>{" "}
                      {this.props.model.rootCauseTargetLabels.has(target.id?.targetCompleted?.label ?? "") && (
                        <span className="root-cause-badge">Root cause</span>
                      )}
                    </div>
                    <div className="target-duration">
                      {this.props.model.getRuntime(target.id?.targetCompleted?.label ?? "")}
                    </div>
                  </Link>
                ))}
            </div>
          </div>
          {this.props.pageSize &&
            events.length > this.props.pageSize * this.state.numPages &&
            !!this.state.numPages && (
              <div className="more" onClick={this.handleMoreClicked.bind(this)}>
                See more {this.props.presentVerb}
              </div>
            )}
        </div>
      </div>
    );
  }
}
