import React from "react";
import { User } from "../auth/auth_service";
import InvocationCompareButton from "./invocation_compare_button";
import InvocationMenuComponent from "./invocation_menu";
import InvocationModel from "./invocation_model";
import InvocationShareButton from "./invocation_share_button";
import WorkflowRerunButton from "./workflow_rerun_button";

export interface InvocationButtonsProps {
  model: InvocationModel;
  invocationId: string;
  user?: User;
}

export default class InvocationButtons extends React.Component<InvocationButtonsProps> {
  private canRerunWorkflow() {
    const repoUrl = this.props.model.getRepo();
    // This repo URL comes from the GitHub API, so no need to worry about
    // ssh or other URL formats.
    return repoUrl.startsWith("https://github.com/");
  }

  render() {
    return (
      <div className="invocation-top-right-buttons">
        {this.props.model.isWorkflowInvocation() && this.canRerunWorkflow() && (
          <WorkflowRerunButton model={this.props.model} />
        )}
        <InvocationCompareButton invocationId={this.props.invocationId} />
        <InvocationShareButton user={this.props.user} model={this.props.model} invocationId={this.props.invocationId} />
        <InvocationMenuComponent
          user={this.props.user}
          model={this.props.model}
          invocationId={this.props.invocationId}
        />
      </div>
    );
  }
}
