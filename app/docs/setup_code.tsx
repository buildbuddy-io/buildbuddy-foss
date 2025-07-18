import React from "react";
import { api_key } from "../../proto/api_key_ts_proto";
import { bazel_config } from "../../proto/bazel_config_ts_proto";
import authService, { User } from "../auth/auth_service";
import capabilities from "../capabilities/capabilities";
import Banner from "../components/banner/banner";
import LinkButton from "../components/button/link_button";
import Select, { Option } from "../components/select/select";
import Spinner from "../components/spinner/spinner";
import error_service from "../errors/error_service";
import rpcService from "../service/rpc_service";

interface Props {
  /** Whether to require the cache to be enabled. */
  requireCacheEnabled?: boolean;
  /** Optional instructions to display above the setup code. */
  instructionsHeader?: React.ReactNode;
}

interface State {
  bazelConfigResponse?: bazel_config.IGetBazelConfigResponse;
  user?: User;
  selectedCredentialIndex: number;
  apiKeyLoading?: boolean;

  auth: "none" | "cert" | "key";
  separateAuth: boolean;
  cache: "read" | "top" | "full";
  cacheChecked: boolean;
  executionChecked: boolean;
}

export default class SetupCodeComponent extends React.Component<Props, State> {
  state: State = {
    selectedCredentialIndex: 0,
    user: authService.user,

    auth: capabilities.auth ? "key" : "none",
    cacheChecked: false,
    cache: "read",
    executionChecked: false,
    separateAuth: false,
  };

  componentWillMount() {
    if (this.props.requireCacheEnabled) this.setState({ cacheChecked: true });

    authService.userStream.subscribe({
      next: (user?: User) => this.setState({ user }),
    });

    let request = new bazel_config.GetBazelConfigRequest();
    request.host = window.location.host;
    request.protocol = window.location.protocol;
    request.includeCertificate = true;
    rpcService.service
      .getBazelConfig(request)
      .then((response) => {
        this.fetchAPIKeyValue(response, 0);
        this.setState({ bazelConfigResponse: response, selectedCredentialIndex: 0 });
      })
      .catch((e) => error_service.handleError(e));
  }

  getSelectedCredential(): bazel_config.Credentials | null {
    const { bazelConfigResponse: response, selectedCredentialIndex: index } = this.state;
    if (!response?.credential) return null;

    return response.credential[index] || null;
  }

  handleInputChange(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    const name = target.name;
    this.setState({
      [name]: target.value,
    } as Record<keyof State, any>);
  }

  handleCheckboxChange(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    const name = target.name;
    this.setState({
      [name]: target.checked,
    } as Record<keyof State, any>);
  }

  getResultsUrl() {
    return this.state.bazelConfigResponse?.configOption?.find(
      (option: bazel_config.IConfigOption) => option.flagName == "bes_results_url"
    )?.body;
  }

  getEventStream() {
    return this.state.bazelConfigResponse?.configOption?.find(
      (option: bazel_config.IConfigOption) => option.flagName == "bes_backend"
    )?.body;
  }

  getCache() {
    return this.state.bazelConfigResponse?.configOption?.find(
      (option: bazel_config.IConfigOption) => option.flagName == "remote_cache"
    )?.body;
  }

  getCacheOptions() {
    if (this.state.cache == "read")
      return (
        <span>
          build --noremote_upload_local_results{" "}
          <span className="comment"># Uploads logs & artifacts without writing to cache</span>
        </span>
      );
    if (this.state.cache == "top")
      return (
        <span>
          build --remote_download_toplevel{" "}
          <span className="comment"># Helps remove network bottleneck if caching is enabled</span>
        </span>
      );
    return "";
  }

  getRemoteOptions() {
    return <span>build --remote_timeout=10m</span>;
  }

  getRemoteExecution() {
    return this.state.bazelConfigResponse?.configOption?.find(
      (option: bazel_config.IConfigOption) => option.flagName == "remote_executor"
    )?.body;
  }

  getCredentials() {
    if (this.state.auth == "cert") {
      return (
        <div>
          <div>build --tls_client_certificate=buildbuddy-cert.pem</div>
          <div>build --tls_client_key=buildbuddy-key.pem</div>
        </div>
      );
    }

    if (this.state.auth == "key") {
      const selectedCredential = this.getSelectedCredential();
      if (!selectedCredential?.apiKey) return null;

      return (
        <div>
          <div>build --remote_header=x-buildbuddy-api-key={selectedCredential.apiKey.value}</div>
        </div>
      );
    }

    return null;
  }

  isAuthEnabled() {
    return Boolean(capabilities.auth && this.state.user);
  }

  isCacheEnabled() {
    return Boolean(
      this.state.bazelConfigResponse?.configOption?.find(
        (option: bazel_config.IConfigOption) => option.flagName == "remote_cache"
      )
    );
  }

  isExecutionEnabled() {
    return (
      (this.isAuthenticated() || !capabilities.auth) &&
      Boolean(
        this.state.bazelConfigResponse?.configOption?.find(
          (option: bazel_config.IConfigOption) => option.flagName == "remote_executor"
        )
      )
    );
  }

  isCertEnabled() {
    const certificate = this.getSelectedCredential()?.certificate;
    return Boolean(certificate && certificate.cert && certificate.key);
  }

  isAuthenticated() {
    return this.isAuthEnabled() && this.state.auth != "none";
  }

  handleCopyClicked(event: any) {
    var copyText = event.target.parentElement.firstChild;
    var input = document.createElement("textarea");
    input.value = copyText.innerText;
    document.body.appendChild(input);
    input.select();
    document.execCommand("copy");
    document.body.removeChild(input);
    event.target.innerText = "Copied!";
  }

  async fetchAPIKeyValue(bazelConfigResponse: bazel_config.IGetBazelConfigResponse, selectedIndex: number) {
    this.setState({ apiKeyLoading: true });
    try {
      const creds = bazelConfigResponse.credential;
      if (!creds?.length) return;

      const selectedCreds = creds[selectedIndex];
      if (!selectedCreds.apiKey || selectedCreds.apiKey.value) return;

      const response = await rpcService.service.getApiKey(
        api_key.GetApiKeyRequest.create({
          apiKeyId: selectedCreds.apiKey.id,
        })
      );
      selectedCreds.apiKey.value = response.apiKey?.value ?? "";
      this.forceUpdate();
    } catch (e) {
      error_service.handleError(e);
    } finally {
      this.setState({ apiKeyLoading: false });
    }
  }

  onChangeCredential(e: React.ChangeEvent<HTMLSelectElement>) {
    const selectedIndex = Number(e.target.value);
    this.fetchAPIKeyValue(this.state.bazelConfigResponse!, selectedIndex);
    this.setState({ selectedCredentialIndex: selectedIndex });
  }

  private getCreateApiKeyLink(): string | null {
    // If the user is an admin (meaning they can create org-level keys), link to the API keys page.
    if (this.state.user?.isGroupAdmin()) return "/settings/org/api-keys";
    // If the user is not an admin but user-level keys are enabled, link to the user-level page.
    if (this.state.user?.selectedGroup.userOwnedKeysEnabled) return "/settings/personal/api-keys";

    return null;
  }

  renderMissingApiKeysNotice() {
    const createLink = this.getCreateApiKeyLink();

    return (
      <div className="no-api-keys">
        <div className="no-api-keys-content">
          <div>
            {capabilities.anonymous ? (
              <>Looks like your organization doesn't have any API keys that are visible to you.</>
            ) : (
              <>This BuildBuddy installation requires authentication, but no API keys are set up.</>
            )}
            {!createLink && <> You do not have permission to create API keys within this organization.</>}
          </div>
          {createLink !== null && (
            <div>
              <LinkButton className="manage-keys-button" href={createLink}>
                Manage keys
              </LinkButton>
            </div>
          )}
        </div>
      </div>
    );
  }

  render() {
    if (!this.state.bazelConfigResponse || this.state.apiKeyLoading) {
      return <Spinner />;
    }
    const selectedCredential = this.getSelectedCredential();

    if (!capabilities.anonymous && !selectedCredential) {
      // TODO(bduffany): Ideally we'd hide the entire setup page and direct the user to set up
      // API keys before taking any further steps.
      return <div className="setup">{this.renderMissingApiKeysNotice()}</div>;
    }

    if (
      this.props.requireCacheEnabled &&
      !this.state.bazelConfigResponse?.configOption?.some((option) => option.flagName === "remote_cache")
    ) {
      return (
        <div className="setup">
          <Banner type="info">Remote caching is not enabled for this BuildBuddy instance.</Banner>
        </div>
      );
    }

    return (
      <div className="setup">
        {this.props.instructionsHeader && <div className="setup-instructions">{this.props.instructionsHeader}</div>}
        <div className="setup-step-header">Select options</div>
        <div className="setup-controls">
          {this.isAuthEnabled() && (
            <span className="group-container">
              <span className="group">
                {capabilities.anonymous && (
                  <input
                    id="auth-none"
                    checked={this.state.auth == "none"}
                    onChange={this.handleInputChange.bind(this)}
                    value="none"
                    name="auth"
                    type="radio"
                  />
                )}
                {capabilities.anonymous && <label htmlFor="auth-none">No auth</label>}
                <input
                  id="auth-key"
                  checked={this.state.auth == "key"}
                  onChange={this.handleInputChange.bind(this)}
                  value="key"
                  name="auth"
                  type="radio"
                />
                <label htmlFor="auth-key">API Key</label>
                {this.isCertEnabled() && (
                  <span className="group-section">
                    <input
                      id="auth-cert"
                      checked={this.state.auth == "cert"}
                      onChange={this.handleInputChange.bind(this)}
                      value="cert"
                      name="auth"
                      type="radio"
                    />
                    <label htmlFor="auth-cert">Certificate</label>
                  </span>
                )}
              </span>
            </span>
          )}

          {(this.state.auth === "cert" || this.state.auth === "key") &&
            (this.state.bazelConfigResponse?.credential?.length || 0) > 1 && (
              <span>
                <Select
                  title="Select API key"
                  className="credential-picker"
                  name="selectedCredential"
                  value={this.state.selectedCredentialIndex}
                  onChange={this.onChangeCredential.bind(this)}>
                  {this.state.bazelConfigResponse.credential?.map((credential, index) => (
                    <Option key={index} value={index}>
                      {credential.apiKey?.label || "Untitled key"}
                      {credential.apiKey?.userOwned && " (personal key)"}
                    </Option>
                  ))}
                </Select>
              </span>
            )}

          {this.isAuthenticated() && (
            <span className="group-container">
              <span className="group">
                <input
                  id="split"
                  checked={this.state.separateAuth}
                  onChange={this.handleCheckboxChange.bind(this)}
                  name="separateAuth"
                  type="checkbox"
                />
                <label htmlFor="split">
                  <span>Separate auth file</span>
                </label>
              </span>
            </span>
          )}

          {this.isCacheEnabled() && (
            <span className="group-container">
              <span className="group">
                <input
                  id="cache"
                  checked={this.state.cacheChecked}
                  onChange={this.handleCheckboxChange.bind(this)}
                  name="cacheChecked"
                  type="checkbox"
                  disabled={this.props.requireCacheEnabled}
                />
                <label htmlFor="cache">
                  <span>Enable cache</span>
                </label>
              </span>
            </span>
          )}

          {this.state.cacheChecked && (
            <span className="group-container">
              <span className="group">
                <input
                  id="cache-full"
                  checked={this.state.cache == "full"}
                  onChange={this.handleInputChange.bind(this)}
                  value="full"
                  name="cache"
                  type="radio"
                />
                <label htmlFor="cache-full">Full cache</label>
                <input
                  id="cache-read"
                  checked={this.state.cache == "read"}
                  onChange={this.handleInputChange.bind(this)}
                  value="read"
                  name="cache"
                  type="radio"
                />
                <label htmlFor="cache-read">Read only</label>
                <input
                  id="cache-top"
                  checked={this.state.cache == "top"}
                  onChange={this.handleInputChange.bind(this)}
                  value="top"
                  name="cache"
                  type="radio"
                />
                <label htmlFor="cache-top">Top level</label>
              </span>
            </span>
          )}

          {this.isExecutionEnabled() && (
            <span className="group-container">
              <span className="group">
                <input
                  id="execution"
                  checked={this.state.executionChecked}
                  onChange={this.handleCheckboxChange.bind(this)}
                  name="executionChecked"
                  type="checkbox"
                />
                <label htmlFor="execution">
                  <span>Enable remote execution</span>
                </label>
              </span>
            </span>
          )}
        </div>
        {this.state.executionChecked && (
          <div className="setup-notice">
            <b>Note:</b> You've enabled remote execution. In addition to these .bazelrc flags, you'll also need to
            configure platforms and toolchains which likely involve modifying your WORKSPACE file. See{" "}
            <b>
              <a target="_blank" href="https://www.buildbuddy.io/docs/rbe-setup">
                our guide on configuring platforms and toolchains
              </a>
            </b>{" "}
            for more info.
          </div>
        )}
        {!(this.isAuthenticated() && !selectedCredential) && (
          <>
            <b>Copy to your .bazelrc</b>
            <code data-header=".bazelrc">
              <div className="contents">
                <div>{this.getResultsUrl()}</div>
                <div>{this.getEventStream()}</div>
                {this.state.cacheChecked && <div>{this.getCache()}</div>}
                {this.state.cacheChecked && <div>{this.getCacheOptions()}</div>}
                {(this.state.cacheChecked || this.state.executionChecked) && <div>{this.getRemoteOptions()}</div>}
                {this.state.executionChecked && <div>{this.getRemoteExecution()}</div>}
                {!this.state.separateAuth && <div>{this.getCredentials()}</div>}
              </div>
              <button onClick={this.handleCopyClicked.bind(this)}>Copy</button>
            </code>
            {this.state.separateAuth && this.isAuthenticated() && (
              <div>
                The file below contains your auth credentials - place it in your home directory at{" "}
                <span className="code">~/.bazelrc</span> or place it anywhere you'd like and add{" "}
                <span className="code">try-import /path/to/your/.bazelrc</span> in your primary{" "}
                <span className="code">.bazelrc</span> file.
                <code data-header="~/.bazelrc">
                  <div className="contents">
                    <div>{this.getCredentials()}</div>
                  </div>
                  <button onClick={this.handleCopyClicked.bind(this)}>Copy</button>
                </code>
              </div>
            )}
            {this.state.auth == "cert" && (
              <div>
                <div className="downloads">
                  {selectedCredential?.certificate?.cert && (
                    <div>
                      <a
                        download="buildbuddy-cert.pem"
                        href={window.URL.createObjectURL(
                          new Blob([selectedCredential.certificate.cert], {
                            type: "text/plain",
                          })
                        )}>
                        Download buildbuddy-cert.pem
                      </a>
                    </div>
                  )}
                  {selectedCredential?.certificate?.key && (
                    <div>
                      <a
                        download="buildbuddy-key.pem"
                        href={window.URL.createObjectURL(
                          new Blob([selectedCredential.certificate.key], {
                            type: "text/plain",
                          })
                        )}>
                        Download buildbuddy-key.pem
                      </a>
                    </div>
                  )}
                </div>
                To use certificate based auth, download the two files above and place them in your workspace directory.
                If you place them outside of your workspace, update the paths in your{" "}
                <span className="code">.bazelrc</span> file to point to the correct location.
                <br />
                <br />
                Note: Certificate based auth is only compatible with Bazel version 3.1 and above.
              </div>
            )}
          </>
        )}
        {this.isAuthenticated() && !selectedCredential && this.renderMissingApiKeysNotice()}
      </div>
    );
  }
}
