import { ArrowDownCircle, FileCode } from "lucide-react";
import React from "react";

import { build_event_stream } from "../../proto/build_event_stream_ts_proto";
import { zip } from "../../proto/zip_ts_proto";
import capabilities from "../capabilities/capabilities";
import DigestComponent from "../components/digest/digest";
import { TextLink } from "../components/link/link";
import rpcService from "../service/rpc_service";
import { getFileDigest } from "../util/cache";

interface Props {
  name: string;
  files: build_event_stream.File[];
  invocationId: string;
}

interface State {
  loading: boolean;
  manifest?: zip.Manifest;
}

export default class TargetArtifactsCardComponent extends React.Component<Props, State> {
  static readonly ZIPPED_OUTPUTS_FILE: string = "test.outputs__outputs.zip";

  state: State = {
    loading: false,
  };

  componentDidMount() {
    this.maybeFetchOutputManifest();
  }

  componentDidUpdate(prevProps: Props) {
    if (this.props.files !== prevProps.files) {
      this.maybeFetchOutputManifest();
    }
  }

  maybeFetchOutputManifest() {
    if (!capabilities.config.testOutputManifestsEnabled) {
      return;
    }
    let testOutputsUri = this.props.files.find(
      (file: build_event_stream.File) => file.name === TargetArtifactsCardComponent.ZIPPED_OUTPUTS_FILE
    )?.uri;

    if (!testOutputsUri || !testOutputsUri.startsWith("bytestream://")) {
      return;
    }

    this.setState({ loading: true });
    const request = new zip.GetZipManifestRequest();
    request.uri = testOutputsUri;
    rpcService.service
      .getZipManifest(request)
      .then((response) => {
        this.setState({ manifest: response.manifest ?? undefined, loading: false });
      })
      .catch(() => {
        this.setState({
          loading: false,
          manifest: undefined,
        });
      });
  }

  encodeManifestEntry(entry: zip.ManifestEntry): string {
    return btoa(
      zip.ManifestEntry.encode(entry)
        .finish()
        .reduce((str, b) => str + String.fromCharCode(b), "")
    );
  }

  makeArtifactUri(baseUri: string, entry: zip.ManifestEntry): string {
    return rpcService.getBytestreamUrl(baseUri, this.props.invocationId, {
      filename: entry.name,
      zip: this.encodeManifestEntry(entry),
    });
  }

  makeArtifactViewUri(baseUri: string, outputFilename: string, encodedZipData?: string): string {
    let params: Record<string, string> = {
      bytestream_url: baseUri,
      invocation_id: this.props.invocationId,
      filename: outputFilename,
    };
    if (encodedZipData) {
      params.z = encodedZipData;
    }
    return `/code/buildbuddy-io/buildbuddy/?${new URLSearchParams(params).toString()}`;
  }

  handleArtifactClicked(outputUri: string, outputFilename: string, event: React.MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    if (!outputUri) return false;

    if (outputUri.startsWith("file://")) {
      window.prompt("Copy artifact path to clipboard: Cmd+C, Enter", outputUri);
    } else if (outputUri.startsWith("bytestream://")) {
      rpcService.downloadBytestreamFile(outputFilename, outputUri, this.props.invocationId);
    }
    return false;
  }

  handleZipArtifactClicked(
    outputUri: string,
    outputFilename: string,
    entry: zip.ManifestEntry,
    event: React.MouseEvent<HTMLAnchorElement>
  ) {
    event.preventDefault();
    if (!outputUri) return false;

    if (outputUri.startsWith("file://")) {
      window.prompt("Copy artifact path to clipboard: Cmd+C, Enter", outputUri);
    } else if (outputUri.startsWith("bytestream://")) {
      rpcService.downloadBytestreamZipFile(
        outputFilename,
        outputUri,
        this.encodeManifestEntry(entry),
        this.props.invocationId
      );
    }
    return false;
  }

  render() {
    return (
      <div className="card artifacts">
        <ArrowDownCircle className="icon brown" />
        <div className="content">
          <div className="title">Artifacts: {this.props.name}</div>
          <div className="details">
            {this.props.files.map((output) => (
              <>
                <div className="artifact-line">
                  <TextLink
                    plain
                    href={rpcService.getBytestreamUrl(output.uri, this.props.invocationId, {
                      filename: output.name,
                    })}
                    className="artifact-name"
                    onClick={this.handleArtifactClicked.bind(this, output.uri, output.name)}>
                    {output.name}
                  </TextLink>
                  {output.uri?.startsWith("bytestream://") && (
                    <a className="artifact-view" href={this.makeArtifactViewUri(output.uri, output.name)}>
                      <FileCode /> View
                    </a>
                  )}
                  <DigestComponent digest={getFileDigest(output) ?? {}} />
                </div>
                {output.name === TargetArtifactsCardComponent.ZIPPED_OUTPUTS_FILE &&
                  this.state.manifest &&
                  this.state.manifest.entry?.map((entry) => (
                    <div className="artifact-line sub-item">
                      <TextLink
                        plain
                        href={this.makeArtifactUri(output.uri, entry)}
                        className="artifact-name"
                        onClick={this.handleZipArtifactClicked.bind(this, output.uri, entry.name, entry)}>
                        {entry.name}
                      </TextLink>
                      {output.uri?.startsWith("bytestream://") && (
                        <a
                          className="artifact-view"
                          href={this.makeArtifactViewUri(output.uri, entry.name, this.encodeManifestEntry(entry))}>
                          <FileCode /> View
                        </a>
                      )}
                    </div>
                  ))}
              </>
            ))}
          </div>
          {this.props.files.length == 0 && <span>No artifacts</span>}
        </div>
      </div>
    );
  }
}
