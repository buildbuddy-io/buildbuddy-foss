---
id: workflows-config
title: Workflows configuration
sidebar_label: Workflows configuration
---

Once you've linked your repo to BuildBuddy via
[BuildBuddy workflows](workflows-setup.md), BuildBuddy will automatically
run `bazel test //...` on each push to your repo, reporting results to the
BuildBuddy UI.

But you may wish to configure multiple test commands with different test
tag filters, or run the same tests on multiple different platform
configurations (running some tests on Linux, and some on macOS, for
example).

This page describes how to configure your workflows beyond the default
configuration.

## Configuring workflow actions and triggers

BuildBuddy workflows can be configured using a file called
`buildbuddy.yaml`, which can be placed at the root of your git repo.

`buildbuddy.yaml` consists of multiple **actions**. Each action describes
a list of commands to be run in order, as well as the set of git
events that should trigger these commands.

:::note

The configuration in `buildbuddy.yaml` only takes effect after you
[enable workflows for the repo](workflows-setup#enable-workflows-for-a-repo).

:::

### Example config

You can copy this example config as a starting point for your own `buildbuddy.yaml`:

```yaml title="buildbuddy.yaml"
actions:
  - name: "Test all targets"
    triggers:
      push:
        branches:
          - "main" # <-- replace "main" with your main branch name
      pull_request:
        branches:
          - "*"
    steps:
      - run: "bazel test //..."
```

This config is equivalent to the default config that we use if you
do not have a `buildbuddy.yaml` file at the root of your repo.

### Running bash commands

Each step can run arbitrary bash code, which may be useful for running Bazel commands
conditionally, or for installing system dependencies
that aren't available in BuildBuddy's available workflow images.

Because workflows are run in [snapshotted microVMs](./rbe-microvms), system
dependencies will be persisted across workflow runs. However, we recommend
fetching dependencies with Bazel whenever possible, rather than relying
on system dependencies.

To specify multiple bash commands, you can either specify a block of bash code within a single step:

```yaml title="buildbuddy.yaml"
# ...
steps:
  - run: |
      sudo apt-get update && sudo apt-get install -y my-lib
      bazel test //...
```

Or you can specify one command per step. Note that each step is run in a separate
bash process, so locally initialized variables will not persist across steps:

```yaml title="buildbuddy.yaml"
# ...
steps:
  - run: sudo apt-get update && sudo apt-get install -y my-lib
  - run: bazel test //...
```

## Bazel configuration

### Bazel version

BuildBuddy runs each bazel command in your workflow with a
[bazelisk](https://github.com/bazelbuild/bazelisk)-compatible wrapper so
that your `.bazelversion` file is respected.

If `.bazelversion` is missing, the latest version of Bazel is used. We
always recommend including a `.bazelversion` in your repo to prevent
problems caused by using conflicting versions of Bazel in different build
environments.

### bazelrc

BuildBuddy runs each bazel command directly in your workspace, which means
that your `.bazelrc` is respected. If you have lots of flags, we recommend
adding them to your `.bazelrc` instead of adding them to your `buildbuddy.yaml`.

BuildBuddy also provides a [`bazelrc`](https://bazel.build/docs/bazelrc)
file which passes these default options to each bazel invocation listed in
`steps`:

- `--bes_backend` and `--bes_results_url`, so that the results from each
  Bazel command are viewable with BuildBuddy
- `--remote_header=x-buildbuddy-api-key=YOUR_API_KEY`, so that invocations
  are authenticated by default
- `--build_metadata=ROLE=CI`, so that workflow invocations are tagged as
  CI invocations, and so that workflow tests are viewable in the test grid

BuildBuddy's `bazelrc` takes lower precedence than your workspace
`.bazelrc`. You can view the exact flags provided by this bazelrc by
inspecting the command line details in the invocation page (look for
`buildbuddy.bazelrc`).

:::note

BuildBuddy remote cache and remote execution (RBE) are not enabled by
default for workflows, and require additional configuration. The
configuration steps are the same as when running Bazel locally. See the
**Quickstart** page in the BuildBuddy UI.

:::

### Secrets

Trusted workflow executions can access [secrets](secrets) using
environment variables.

For example, if we have a secret named `REGISTRY_TOKEN` and we want to set
the remote header `x-buildbuddy-platform.container-registry-password` to
the value of that secret, we can get the secret value using
`$REGISTRY_TOKEN`, as in the following example:

```yaml title="buildbuddy.yaml"
# ...
steps:
  - run: "bazel test ... --remote_exec_header=x-buildbuddy-platform.container-registry-password=$REGISTRY_TOKEN"
```

To access the environment variables within `build` or `test` actions, you
may need to explicitly expose the environment variable to the actions by
using a bazel flag like
[`--action_env`](https://bazel.build/reference/command-line-reference#flag--action_env)
or
[`--test_env`](https://bazel.build/reference/command-line-reference#flag--test_env):

```yaml title="buildbuddy.yaml"
# ...
steps:
  - run: "bazel test ... --test_env=REGISTRY_TOKEN"
```

## Merge queue support

BuildBuddy workflows are compatible with GitHub's [merge queues](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue).

To ensure that workflows are run as part of merge queue CI, configure
a push trigger that runs whenever GitHub pushes its temporary merge queue
branch, as described in [Triggering merge group checks with third-party CI providers](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue).

Example `buildbuddy.yaml` file:

```yaml title="buildbuddy.yaml"
- action: Test
  triggers:
    push:
      # Run when a merge queue branch is pushed or the main branch is
      # pushed.
      branches: ["main", "gh-readonly-queue/*"]
  # ...
```

## Linux image configuration

By default, workflows run on an Ubuntu 18.04-based image. You can
customize the image using the `container_image` action setting:

```yaml title="buildbuddy.yaml"
actions:
  - name: "Test all targets"
    container_image: "ubuntu-20.04" # <-- add this line
    steps:
      - run: "bazel test //..."
```

The supported values for `container_image` are:

- `"ubuntu-18.04"` (the default)
- `"ubuntu-20.04"`
- `"ubuntu-22.04"`

## Linux resource configuration

By default, Linux workflow VMs have the following resources available:

- 3 CPU
- 8 GB of RAM
- 20 GB of disk space

These values are configurable using [resource requests](#resourcerequests).

## Mac configuration

By default, workflows will execute on BuildBuddy's Linux executors,
but it is also possible to run workflows on macOS by using self-hosted
executors.

1. Set up one or more Mac executors that will be dedicated to running
   workflows, following the steps in the [Enterprise
   Mac RBE Setup](/docs/enterprise-mac-rbe) guide.

   Then, in your `buildbuddy-executor.plist` file, find the
   `EnvironmentVariables` section and set `MY_POOL` to `workflows`. You'll
   also need to set `SYS_MEMORY_BYTES` to allow enough memory to be
   used for workflows (a minimum of 8GB is required).

```xml title="buildbuddy-executor.plist"
        ...
        <key>EnvironmentVariables</key>
        <dict>
            ...
            <!-- Set the required executor pool name for workflows -->
            <key>MY_POOL</key>
            <string>workflows</string>
            <!-- Allocate 16GB of memory to workflows (8GB minimum) -->
            <key>SYS_MEMORY_BYTES</key>
            <string>16000000000</string>
        </dict>
        ...
```

2. If you haven't already, [enable workflows for your
   repo](/docs/workflows-setup#enable-workflows-for-a-repo), then create a
   file called `buildbuddy.yaml` at the root of your repo. See the
   [Example config](#example-config) for a starting point.

3. Set `os: "darwin"` on the workflow action that you would like to build
   on macOS. For Apple silicon (ARM-based) Macs, add `arch: "arm64"` as
   well. Note: if you copy another action as a starting point, be sure to
   give the new action a unique name:

```yaml title="buildbuddy.yaml"
actions:
  - name: "Test all targets (Mac)"
    os: "darwin" # <-- add this line
    arch: "arm64" # <-- add this line for Apple silicon (ARM-based) Macs only
    triggers:
      push:
        branches:
          - "main"
      pull_request:
        branches:
          - "*"
    steps:
      - run: "bazel test //..."
```

That's it! Whenever any of the configured triggers are matched, one of
the Mac executors in the `workflows` pool should execute the
workflow, and BuildBuddy will publish the results to your branch.

## buildbuddy.yaml schema

### `BuildBuddyConfig`

The top-level BuildBuddy workflow config, which specifies bazel commands
that can be run on a repo, as well as the events that trigger those commands.

**Fields:**

- **`actions`** ([`Action`](#action) list): List of actions that can be triggered by BuildBuddy.
  Each action corresponds to a separate check on GitHub.
  If multiple actions are matched for a given event, the actions are run in
  order. If an action fails, subsequent actions will still be executed.

### `Action`

A named group of Bazel commands that run when triggered.

**Fields:**

- **`name`** (`string`): A name unique to this config, which shows up as the name of the check
  in GitHub.
- **`triggers`** ([`Triggers`](#triggers)): The triggers that should cause this action to be run.
- **`os`** (`string`): The operating system on which to run the workflow.
  Defaults to `"linux"`. `"darwin"` (macOS) is also supported, but
  requires using self-hosted Mac executors running on a dedicated
  `workflows` pool.
- **`arch`** (`string`): The CPU architecture of the workflow runner.
  Defaults to `"amd64"`. `"arm64"` is also supported when running under
  `os: "darwin"`, but requires using self-hosted Apple silicon (ARM-based)
  Mac executors running on a dedicated `workflows` pool.
- **`pool`** (`string`): The executor pool name for running workflows.
  This option has no effect unless `self_hosted: true` is also specified.
- **`self_hosted`** (`boolean`): Whether to run the workflow on
  self-hosted executors. The executor's default isolation type will be
  used to run workflows. Unless `pool` is also specified, the configured
  pool name for the self-hosted workflow executors must be `"workflows"`.
  This option is ignored for macOS workflows, since macOS workflows are
  always required to be self-hosted.
- **`container_image`** (`string`): The Linux container image to use
  (has no effect for Mac workflows). Supported values are `"ubuntu-18.04"`
  and `"ubuntu-20.04"`. Defaults to `"ubuntu-18.04"`.
- **`resource_requests`** ([`ResourceRequests`](#resourcerequests)):
  the requested resources for this action.
- **`user`** (`string`): User to run the workflow as. This can be set to
  `"root"` to run the workflow as root, but it is recommended to keep the
  default value, which is a non-root user provisioned in the CI
  environment (usually named `"buildbuddy"`). Note: some legacy workflows
  might still have `"root"` as the default user, but we are in the process
  of migrating all users to non-root by default.
- **`env`** (`map` with string values): Map of static environment variables and their values.
- **`git_fetch_filters`** (`string` list): list of [`--filter` option](https://git-scm.com/docs/git-clone#Documentation/git-clone.txt-code--filtercodeemltfilter-specgtem)
  values to the `git fetch` command used when fetching the git commits
  to build. Defaults to `["blob:none"]`.
- **`git_fetch_depth`** (`int`): [`--depth` option](https://git-scm.com/docs/git-fetch#Documentation/git-fetch.txt---depthltdepthgt) value used when
  fetching the git commits to build. When using this option in combination
  with a `pull_request` trigger, it's recommended to set
  `merge_with_base: false` in the `pull_request` trigger, since the
  limited fetch depth might prevent the merge-base commit from being
  fetched. Defaults to `0` (unset).
- **`git_clean_exclude`** (`string` list): List of directories within the
  workspace that are excluded when running `git clean` across actions that
  are executed in the same runner instance. This is an advanced option and
  is not recommended for most users.
- **`bazel_workspace_dir`** (`string`): A subdirectory within the repo
  containing the bazel workspace for this action. By default, this is
  assumed to be the repo root directory.
- **`steps`** (list): Bash commands to be run in order.
  If a command fails, subsequent ones are not run, and the action is
  reported as failed. Otherwise, the action is reported as succeeded.
  Environment variables are expanded, which means that the commands
  can reference [secrets](secrets.md) if the workflow execution
  is trusted.
- **`timeout`** (`duration` string, e.g. '30m', '1h'): If set, workflow actions that have been
  running for longer than this duration will be canceled automatically. This
  only applies to a single invocation, and does not include multiple retry attempts.

### `Triggers`

Defines whether an action should run when a branch is pushed to the repo.

**Fields:**

- **`push`** ([`PushTrigger`](#pushtrigger)): Configuration for push events associated with the repo.
  This is mostly useful for reporting commit statuses that show up on the
  home page of the repo.
- **`pull_request`** ([`PullRequestTrigger`](#pullrequesttrigger)):
  Configuration for pull request events associated with the repo.
  This is required if you want to use BuildBuddy to report the status of
  this action on pull requests, and optionally prevent pull requests from
  being merged if the action fails.

### `PushTrigger`

Defines whether an action should execute when a branch is pushed.

**Fields:**

- **`branches`** (`string` list): The branches that, when pushed to, will
  trigger the action. This field accepts a simple wildcard character
  (`"*"`) as a possible value, which will match any branch, as well as
  `"gh-readonly-queue/*"`, which matches GitHub's merge queue branches.

### `PullRequestTrigger`

Defines whether an action should execute when a pull request (PR) branch is
pushed.

**Fields:**

- **`branches`** (`string` list): The _base_ branches of a pull request.
  For example, if this is set to `[ "v1", "v2" ]`, then the associated
  action is only run when a PR wants to merge a branch _into_ the `v1`
  branch or the `v2` branch. This field accepts a simple wildcard
  character (`"*"`) as a possible value, which will match any branch.
- **`merge_with_base`** (`boolean`, default: `true`): Whether to merge the
  base branch into the PR branch before running the workflow action. This
  can help ensure that the changes in the PR branch do not conflict with
  the main branch. However, the action will not be continuously re-run as
  changes are pushed to the base branch. For stronger protection against
  breaking the main branch, you may wish to use [merge
  queues](#merge-queue-support).

### `ResourceRequests`

Defines the requested resources for a workflow action.

**Fields:**

- **`memory`** (`string` or `number`): Requested amount of memory for the
  workflow action. Can be specified as an exact number of bytes, or a
  numeric string containing an IEC unit abbreviation. For example: `8GB`
  represents `8 * (1024)^3` bytes of memory.
- **`cpu`** (`string` or `number`): Requested amount of CPU for the
  workflow action. Can be specified as a number of CPU cores, or a numeric
  string containing an `m` suffix to represent thousandths of a CPU core.
  For example: `8000m` represents 8 CPU cores.
- **`disk`** (`string` or `number`): Requested amount of disk space for the
  workflow action. Can be specified as a number of bytes, or a numeric
  string containing an IEC unit abbreviation. For example: `8GB` represents
  `8 * (1024)^3` bytes of disk space.
