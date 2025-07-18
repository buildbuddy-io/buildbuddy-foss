import { HelpCircle } from "lucide-react";
import React from "react";
import { target } from "../../proto/target_ts_proto";
import { OutlinedButton } from "../components/button/button";
import { OutlinedLinkButton } from "../components/button/link_button";
import Spinner from "../components/spinner/spinner";
import { Path } from "../router/router";
import rpc_service from "../service/rpc_service";

interface Props {
  repo: string;
  labels: string[];
}

interface State {
  loading: boolean;
  response?: target.GetTargetStatsResponse;
}

export default class FlakyTargetChipComponent extends React.Component<Props, State> {
  state: State = { loading: true };

  componentDidMount() {
    rpc_service.service
      .getTargetStats({ repo: this.props.repo, labels: this.props.labels })
      .then((r) => this.setState({ response: r }))
      .finally(() => this.setState({ loading: false }));
  }
  render() {
    if (this.state.loading) {
      return (
        <OutlinedButton
          disabled={true}
          title={"Determining whether or not this test has been previously flaky."}
          className="flaky-target-chip">
          <Spinner className="icon" /> Checking flakes...
        </OutlinedButton>
      );
    }

    const flakes = this.state.response?.stats
      .filter((v) => v.data?.flakyRuns || v.data?.likelyFlakyRuns)
      .map((v) => v.label);
    if (flakes && flakes.length > 0) {
      const targets = flakes.join(" ");
      const title =
        this.props.labels.length === 1
          ? "This target was recently flaky--click to see samples."
          : "Some failed targets were recently flaky--click to see samples.";
      const href =
        this.props.labels.length === 1
          ? `${Path.tapPath}?target=${encodeURIComponent(targets)}&days=7#flakes`
          : `${Path.tapPath}?targetFilter=${encodeURIComponent(targets)}&days=7#flakes`;
      return (
        <OutlinedLinkButton href={href} title={title} className="flaky-target-chip">
          <HelpCircle className="icon orange" /> Recently flaky
        </OutlinedLinkButton>
      );
    }

    // Didn't find flakes, hide the chip.
    return <></>;
  }
}
