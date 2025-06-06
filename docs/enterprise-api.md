---
id: enterprise-api
title: API Documentation
sidebar_label: Enterprise API
---

The BuildBuddy API let's you programmatically obtain information about your Bazel builds. API access available to Enterprise BuildBuddy Customers.

Requests can be made via JSON or using Protobuf. The examples below are using the JSON API. For a full overview of the service, you can view the [service definition](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/service.proto) or the [individual protos](https://github.com/buildbuddy-io/buildbuddy/tree/master/proto/api/v1).

## GetInvocation

The `GetInvocation` endpoint allows you to fetch invocations associated with a commit SHA or invocation ID. View full [Invocation proto](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/invocation.proto).

### Endpoint

```
https://app.buildbuddy.io/api/v1/GetInvocation
```

### Service

```protobuf
// Retrieves a list of invocations or a specific invocation matching the given
// request selector.
rpc GetInvocation(GetInvocationRequest) returns (GetInvocationResponse);
```

### Example cURL request

```bash
curl -d '{"selector": {"invocation_id":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845"}}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/GetInvocation
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the invocation ID `c6b2b6de-c7bb-4dd9-b7fd-a530362f0845` with your own values.

### Example cURL response

```js
{
  "invocation": [
    {
      "id": {
        "invocationId": "c7fbfe97-8298-451f-b91d-722ad91632ea"
      },
      "success": true,
      "user": "runner",
      "durationUsec": "221970000",
      "host": "fv-az278-49",
      "command": "build",
      "pattern": "//...",
      "actionCount": "1402",
      "createdAtUsec": "1623193638545989",
      "updatedAtUsec": "1623193638545989",
      "repoUrl": "https://github.com/buildbuddy-io/buildbuddy",
      "commitSha": "800f549937a4c0a1614e65501caf7577d2a00624",
      "role": "CI"
    }
  ]
}
```

### GetInvocationRequest

```protobuf
// Request passed into GetInvocation
message GetInvocationRequest {
  // The selector defining which invocations(s) to retrieve.
  InvocationSelector selector = 1;

  // If true, includes additional build metadata.
  bool include_metadata = 2;

  // The next_page_token value returned from a previous request, if any.
  string page_token = 3;

  // If true, include artifacts attached to the invocation.
  bool include_artifacts = 4;
}
```

### GetInvocationResponse

```protobuf
// Response from calling GetInvocation
message GetInvocationResponse {
  // Invocations matching the request invocation, possibly capped by a
  // server limit.
  repeated Invocation invocation = 1;

  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}
```

### InvocationSelector

```protobuf
// The selector used to specify which invocations to return.
message InvocationSelector {
  // One invocation_id or commit_sha is required.

  // Optional: The Invocation ID.
  // Return only the invocation with this invocation ID.
  string invocation_id = 1;

  // Optional: The commmit SHA.
  // If set, only the invocations with this commit SHA will be returned.
  string commit_sha = 2;
}
```

### Invocation

```protobuf
// Response from calling GetInvocation
message GetInvocationResponse {
  // Invocations matching the request invocation, possibly capped by a
  // server limit.
  repeated Invocation invocation = 1;

  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}

// Each Invocation represents metadata associated with a given invocation.
message Invocation {
  // The resource ID components that identify the Invocation.
  message Id {
    // The Invocation ID.
    string invocation_id = 1;
  }

  // The resource ID components that identify the Invocation.
  Id id = 1;

  // Whether or not the build was successful.
  bool success = 3;

  // The user who performed this build.
  string user = 4;

  // The duration of this build, from start to finish.
  int64 duration_usec = 5;

  // The host this build was executed on.
  string host = 6;

  // The command performed (usually "build" or "test").
  string command = 7;

  // The build patterns specified for this build.
  string pattern = 8;

  // The number of actions performed.
  int64 action_count = 9;

  // The time this invocation was created and updated, respectively. Invocations
  // are created as soon as the first event is received from the client and
  // updated with subsequent events until they are finalized.
  int64 created_at_usec = 13;
  int64 updated_at_usec = 14;

  // A URL to the git repo this invocation was for.
  string repo_url = 15;

  // The commit SHA that this invocation was for.
  string commit_sha = 16;

  // The role played by this invocation. Ex: "CI"
  string role = 19;

  // The git branch that this invocation was for.
  string branch_name = 20;

  // The invocation's build metadata. Only included if include_metadata = true.
  repeated InvocationMetadata build_metadata = 21;

  // The invocation's workspace status.
  // Only included if include_metadata = true.
  repeated InvocationMetadata workspace_status = 22;

  // Bazel exit code name for the invocation.
  // At the time of writing, valid exit code names are listed here:
  // https://github.com/bazelbuild/bazel/blob/b3602eb14cf27494a0a754bc215ec2b94d13d89b/src/main/java/com/google/devtools/build/lib/util/ExitCode.java#L42-L72
  // Ex: "INTERRUPTED".
  string bazel_exit_code = 23;

  // Any artifacts that were attached to this invocation.
  // Only included if include_artifacts = true.
  repeated File artifacts = 24;

  // The state of the build event stream for the invocation.
  InvocationStatus invocationStatus = 25;
}

// Key value pair containing invocation metadata.
message InvocationMetadata {
  string key = 1;
  string value = 2;
}

// InvocationStatus represents the state of the build event stream.
enum InvocationStatus {
  UNKNOWN_INVOCATION_STATUS = 0;
  // The build is complete and the Finished BES event has been processed.
  COMPLETE_INVOCATION_STATUS = 1;
  // The build is still in progress, and the stream is still being processed.
  PARTIAL_INVOCATION_STATUS = 2;
  // The stream was broken.
  DISCONNECTED_INVOCATION_STATUS = 3;
}
```

## GetLog

The `GetLog` endpoint allows you to fetch build logs associated with an invocation ID. View full [Log proto](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/log.proto).

### Endpoint

```
https://app.buildbuddy.io/api/v1/GetLog
```

### Service

```protobuf
// Retrieves the logs for a specific invocation.
rpc GetLog(GetLogRequest) returns (GetLogResponse);
```

### Example cURL request

```bash
curl -d '{"selector": {"invocation_id":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845"}}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/GetLog
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the invocation ID `c6b2b6de-c7bb-4dd9-b7fd-a530362f0845` with your own values.

### Example cURL response

```json
{
  "log": {
    "contents": "\u001b[32mINFO: \u001b[0mStreaming build results to: https://app.buildbuddy.io/invocation/c6b2b6de-c7bb-4dd9-b7fd-a530362f0845\n\u001b[33mDEBUG: \u001b[0m/private/var/tmp/_bazel_siggi/d74b389565ce91f59e5b1330988b81f0/external/io_grpc_grpc_java/java_grpc_library.bzl:195:14: Multiple values in 'deps' is deprecated in google_devtools_remoteexecution_v1test_remote_execution_java_grpc\n\u001b[33mDEBUG: \u001b[0m/private/var/tmp/_bazel_siggi/d74b3895654ce91f9e5b1300988b81f0/external/io_grpc_grpc_java/java_grpc_library.bzl:195:14: Multiple values in 'deps' is deprecated in remote_execution_java_grpc\n\u001b[33mDEBUG: \u001b[0m/private/var/tmp/_bazel_siggi/d74b3895654ce91f9e5b1300988b81f0/external/io_grpc_grpc_java/java_grpc_library.bzl:82:14: in srcs attribute of @remoteapis//:remote_execution_java_grpc: Proto source with label @remoteapis//build/bazel/remote/execution/v2:remote_execution_proto should be in same package as consuming rule\n\u001b[32mINFO: \u001b[0mAnalyzed 9 targets (52 packages loaded, 1700 targets configured).\n\u001b[32mINFO: \u001b[0mFound 9 targets...\n\u001b[32mINFO: \u001b[0mFrom Generating Descriptor Set proto_library @googleapis//:google_watch_v1_proto:\ngoogle/watcher/v1/watch.proto:21:1: warning: Import google/protobuf/empty.proto but not used.\n\u001b[32mINFO: \u001b[0mElapsed time: 2.615s, Critical Path: 1.21s\n\u001b[32mINFO: \u001b[0m32 processes: 16 internal, 11 darwin-sandbox, 5 worker.\n\u001b[32mINFO:\u001b[0m Build completed successfully, 32 total actions\n"
  }
}
```

### GetLogRequest

```protobuf
// Request passed into GetLog
message GetLogRequest {
  // The selector defining which logs(s) to retrieve.
  LogSelector selector = 1;

  // The next_page_token value returned from a previous request, if any.
  string page_token = 3;
}
```

### GetLogResponse

```protobuf
// Response from calling GetLog
message GetLogResponse {
  // Log matching the request, possibly capped by a server limit.
  Log log = 1;

  // Token to retrieve the next page of the log, or empty if there are no
  // more logs.
  string next_page_token = 2;
}
```

### LogSelector

```protobuf
// The selector used to specify which logs to return.
message LogSelector {
  // Required: The Invocation ID.
  // Return only the logs associated with this invocation ID.
  string invocation_id = 1;
}
```

### Log

```protobuf
// Each Log represents a chunk of build logs.
message Log {
  // The resource ID components that identify the Log.
  message Id {
    // The Invocation ID.
    string invocation_id = 1;
  }

  // The resource ID components that identify the Log.
  Id id = 1;

  // The contents of the log.
  string contents = 3;
}
```

## GetTarget

The `GetTarget` endpoint allows you to fetch targets associated with a given invocation ID. View full [Target proto](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/target.proto).

### Endpoint

```
https://app.buildbuddy.io/api/v1/GetTarget
```

### Service

```protobuf
// Retrieves a list of targets or a specific target matching the given
// request selector.
rpc GetTarget(GetTargetRequest) returns (GetTargetResponse);
```

### Example cURL request

```bash
curl -d '{"selector": {"invocation_id":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845"}}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/GetTarget
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the invocation ID `c6b2b6de-c7bb-4dd9-b7fd-a530362f0845` with your own values.

### Example cURL response

```js
{
  "target": [
    {
      "id": {
        "invocationId": "c7fbfe97-8298-451f-b91d-722ad91632ea",
        "targetId": "aWQ6OnYxOjovL3Rvb2xzL3JlcGxheV9hY3Rpb246cmVwbGF5X2FjdGlvbl9saWI"
      },
      "label": "//tools/replay_action:replay_action_lib",
      "status": "BUILT",
      "ruleType": "go_library",
      "language": "go"
    },
    ...{
      "id": {
        "invocationId": "c7fbfe97-8298-451f-b91d-722ad91632ea",
        "targetId": "aWQ6OnYxOjovL2VudGVycHJpc2UvYXBwOmNvcHlfYXBwX2J1bmRsZV9zb3VyY2VtYXA"
      },
      "label": "//enterprise/app:copy_app_bundle_sourcemap",
      "status": "BUILT",
      "ruleType": "genrule"
    },
    {
      "id": {
        "invocationId": "c7fbfe97-8298-451f-b91d-722ad91632ea",
        "targetId": "aWQ6OnYxOjovL2VudGVycHJpc2U6YnVpbGRidWRkeQ"
      },
      "label": "//enterprise:buildbuddy",
      "status": "BUILT",
      "ruleType": "go_binary",
      "language": "go"
    }
  ]
}
```

### GetTargetRequest

```protobuf
// Request passed into GetTarget
message GetTargetRequest {
  // The selector defining which target(s) to retrieve.
  TargetSelector selector = 1;

  // The next_page_token value returned from a previous request, if any.
  string page_token = 3;
}
```

### GetTargetResponse

```protobuf
// Response from calling GetTarget
message GetTargetResponse {
  // Targets matching the request invocation, possibly capped by a
  // server limit.
  repeated Target target = 1;

  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}
```

### TargetSelector

```protobuf
// The selector used to specify which targets to return.
message TargetSelector {
  // Required: The Invocation ID.
  // All actions returned will be scoped to this invocation.
  string invocation_id = 1;

  // Optional: The Target ID.
  // If set, only the target with this target id will be returned.
  string target_id = 2;

  // Optional: Tag
  // If set, only targets with this tag will be returned.
  string tag = 3;

  // Optional: The Target label.
  // If set, only the target with this target label will be returned.
  string label = 4;
}
```

### Target

```protobuf
// Each Target represents data for a given target in a given Invocation.
message Target {
  // The resource ID components that identify the Target.
  message Id {
    // The Invocation ID.
    string invocation_id = 1;

    // The Target ID.
    string target_id = 2;
  }

  // The resource ID components that identify the Target.
  Id id = 1;

  // The label of the target Ex: //server/test:foo
  string label = 2;

  // The aggregate status of the target.
  Status status = 3;

  // When this target started and its duration.
  Timing timing = 4;

  // The type of the target rule. Ex: java_binary
  string rule_type = 5;

  // Tags applied to this target (if any).
  repeated string tag = 6;

  // The language of the target rule. Ex: java, go, sh
  string language = 7;
}
```

## GetAction

The `GetAction` endpoint allows you to fetch actions associated with a given target or invocation. View full [Action proto](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/action.proto).

### Endpoint

```
https://app.buildbuddy.io/api/v1/GetAction
```

### Service

```protobuf
// Retrieves a list of targets or a specific target matching the given
// request selector.
rpc GetAction(GetActionRequest) returns (GetActionResponse);
```

### Example cURL request

```bash
curl -d '{"selector": {"invocation_id":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845"}}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/GetAction
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the invocation ID `c6b2b6de-c7bb-4dd9-b7fd-a530362f0845` with your own values.

### Example cURL response

```js
{
   "action":[
      {
         "id":{
            "invocationId":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845",
            "targetId":"aWQ6OnYxOjovLzpzdGF0aWM",
            "configurationId":"00e90e1ab7325d5e63d03bfe5f808477b2bb66ca6ae9af26c036cae67ee81cf9",
            "actionId":"aWQ6OnYxOjpidWlsZA"
         }
      },
      [...]
      {
         "id":{
            "invocationId":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845",
            "targetId":"aWQ6OnYxOjovL2VudGVycHJpc2UvYXBwOnN0eWxl",
            "configurationId":"00e90e1ab7325d5e63d03bfe5f808477b2bb66ca6ae9af26c036cae67ee81cf9",
            "actionId":"aWQ6OnYxOjpidWlsZA"
         },
         "file":[
            {
               "name":"enterprise/app/style.css",
               "uri":"bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/e21b1e3411792e17e698be879a3548527d620c65953986c96d5a81f933e776aa/68837",
               "hash":"e21b1e3411792e17e698be879a3548527d620c65953986c96d5a81f933e776aa",
               "sizeBytes":68837
            }
         ]
      },
      {
         "id":{
            "invocationId":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845",
            "targetId":"aWQ6OnYxOjovLzp2ZXQ",
            "configurationId":"9a01374ae0e8164eec90f708e7a997520994e71b433a5265c89582c4490d75e9",
            "actionId":"aWQ6OnYxOjpidWlsZA"
         },
         "file":[
            {
               "name":"vet_/vet",
               "uri":"bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/915edf6aca4bd4eac3e4602641b0633a7aaf038d62d5ae087884a2d8acf0926a/7029420",
               "hash":"915edf6aca4bd4eac3e4602641b0633a7aaf038d62d5ae087884a2d8acf0926a",
               "sizeBytes":7029420,
            }
         ]
      },
      {
         "id":{
            "invocationId":"c6b2b6de-c7bb-4dd9-b7fd-a530362f0845",
            "targetId":"aWQ6OnYxOjovL2VudGVycHJpc2Uvc2VydmVyL3Rlc3QvaW50ZWdyYXRpb24vcmVtb3RlX2NhY2hlOnJlbW90ZV9jYWNoZV90ZXN0",
            "configurationId":"00e90e1ab7325d5e63d03bfe5f808477b2bb66ca6ae9af26c036cae67ee81cf9",
            "actionId":"aWQ6OnYxOjp0ZXN0LVNfMy1SXzEtQV8x"
         },
         "file":[
            {
               "name":"test.log",
               "uri":"bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216",
               "hash":"09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888",
               "sizeBytes":216
            }
         ]
      }
   ]
}
```

### GetActionRequest

```protobuf
// Request passed into GetAction
message GetActionRequest {
  // The selector defining which action(s) to retrieve.
  ActionSelector selector = 1;

  // The next_page_token value returned from a previous request, if any.
  string page_token = 2;
}
```

### GetActionResponse

```protobuf
// Response from calling GetAction
message GetActionResponse {
  // Actions matching the request, possibly capped a server limit.
  repeated Action action = 1;

  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}
```

### ActionSelector

```protobuf
// The selector used to specify which actions to return.
message ActionSelector {
  // Required: The Invocation ID.
  // All actions returned will be scoped to this invocation.
  string invocation_id = 1;

  // Optional: The Target ID.
  // If set, all actions returned will be scoped to this target.
  string target_id = 2;

  // Optional: The Configuration ID.
  // If set, all actions returned will be scoped to this configuration.
  string configuration_id = 3;

  // Optional: The Action ID.
  // If set, only the action with this action id will be returned.
  string action_id = 4;
}
```

### Action

```protobuf
// An action that happened as part of a configured target. This action could be
// a build, a test, or another type of action.
message Action {
  // The resource ID components that identify the Action.
  message Id {
    // The Invocation ID.
    string invocation_id = 1;

    // The Target ID.
    string target_id = 2;

    // The Configuration ID.
    string configuration_id = 3;

    // The Action ID.
    string action_id = 4;
  }

  // The resource ID components that identify the Action.
  Id id = 1;

  // A list of file references for action level files.
  repeated File file = 2;
}
```

## GetFile

The `GetFile` endpoint allows you to fetch files associated with a given url. View full [File proto](https://github.com/buildbuddy-io/buildbuddy/blob/master/proto/api/v1/file.proto).

### Endpoint

```
https://app.buildbuddy.io/api/v1/GetFile
```

### Service

```protobuf
// Streams the File with the given uri.
// - Over gRPC returns a stream of bytes to be stitched together in order.
// - Over HTTP this simply returns the requested file.
rpc GetFile(GetFileRequest) returns (stream GetFileResponse);
```

### Example cURL request

```bash
curl -d '{"uri":"bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216"}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/GetFile
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the file uri `bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216` with your own values.

### Example cURL response

The file contents.

```
exec ${PAGER:-/usr/bin/less} "$0" || exit 1
Executing tests from //enterprise/server/test/integration/remote_cache:remote_cache_test
-----------------------------------------------------------------------------
PASS
```

### GetFileRequest

```protobuf
// Request object for GetFile
message GetFileRequest {
  // File URI corresponding to the `uri` field in the File message.
  //
  // If the BuildBuddy instance supports ZSTD transcoding, the literal string
  // "/blobs/" in the URI (third-to-last path segment) may be replaced with
  // "/compressed-blobs/zstd/", and the server will return a compressed payload.
  //
  // Examples:
  // * Uncompressed blob with remote instance name of "ci":
  //   bytestream://remote.buildbuddy.io/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216
  //
  // * zstd-compressed blob with no remote instance name:
  //   bytestream://remote.buildbuddy.io/compressed-blobs/zstd/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216
  string uri = 1;
}
```

### GetFileResponse

```protobuf
// Response object for GetFile
message GetFileResponse {
  // The file data.
  bytes data = 1;
}
```

### File

```protobuf
// A file associated with a BuildBuddy build.
message File {
  string name = 1;
  string uri = 2;
  string hash = 3;
  int64 size_bytes = 4;
}
```

## DeleteFile

The `DeleteFile` endpoint allows you to delete a specific cache entry, which is associated with a uri.
This can be used to address cache poisoning.

### Endpoint

```
https://app.buildbuddy.io/api/v1/DeleteFile
```

### Service

```protobuf
// Delete the File with the given uri.
rpc DeleteFile(DeleteFileRequest) returns (DeleteFileResponse);
```

### Example cURL request

```bash
curl -d '{"uri":"bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216"}' \
  -H 'x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY' \
  -H 'Content-Type: application/json' \
  https://app.buildbuddy.io/api/v1/DeleteFile
```

Make sure to replace `YOUR_BUILDBUDDY_API_KEY` and the file uri `bytestream://remote.buildbuddy.io/buildbuddy-io/buildbuddy/ci/blobs/09e6fe6e1fd8c8734339a0a84c3c7a0eb121b57a45d21cfeb1f265bffe4c4888/216` with your own values.

### DeleteFileRequest

```protobuf
// Request object for DeleteFile
message DeleteFileRequest {
  // URI of file to delete.
  //
  // CAS URI format:
  // <instance_name>/<blobs|compressed-blobs/zstd>/<digest_hash>/<digest_size>
  // Action cache URI format:
  // <instance_name>/<blobs|compressed-blobs/zstd>/ac/<digest_hash>/<digest_size>
  //
  // Examples:
  // * CAS artifact:
  //   compressed-blobs/zstd/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/2084
  //
  // * CAS artifact with remote_instance_name
  //   my_remote_instance_name/blobs/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/2084
  //
  // * Action cache artifact:
  //   blobs/ac/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/2084
  //
  // * Action cache artifact with remote_instance_name
  //   my_remote_instance_name/blobs/ac/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/2084
  string uri = 1;
}
```

### DeleteFileResponse

```protobuf
// Response object for DeleteFile
message DeleteFileResponse {}
```

## ExecuteWorkflow

The `ExecuteWorkflow` endpoint lets you trigger a Buildbuddy Workflow for the given repository and branch/commit.

Note: Github App authentication is required. The API does not support running legacy workflows. See
https://www.buildbuddy.io/docs/workflows-setup/ for more information on how to setup workflows with
Github App authentication.

### Endpoint

```
https://app.buildbuddy.io/api/v1/ExecuteWorkflow
```

### Service

```protobuf
rpc ExecuteWorkflow(ExecuteWorkflowRequest) returns (ExecuteWorkflowResponse);
```

### Example cURL request

```bash
# Execute workflow on commit abc123 on branch cool-feature with env vars set
curl -d '{
  "repo_url": "https://github.com/buildbuddy-io/buildbuddy-ci-playground",
  "branch": "cool-feature",
  "commit_sha": "abc123",
  "action_names": ["Test"],
  "env": {"USE_BAZEL_VERSION": "6.4.0"}
}' \
-H "x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY" \
-H 'Content-Type: application/json' \
https://app.buildbuddy.io/api/v1/ExecuteWorkflow

# Example populating `branch` and `commit_sha` from a Github Actions workflow
curl -d '{
  "repo_url": "https://github.com/buildbuddy-io/buildbuddy-ci-playground",
  "branch": "${{ github.ref_name }}",
  "commit_sha": "${{ github.sha }}",
  "action_names": ["Test"],
}' \
-H "x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY" \
-H 'Content-Type: application/json' \
https://app.buildbuddy.io/api/v1/ExecuteWorkflow
```

### ExecuteWorkflowRequest

```protobuf
message ExecuteWorkflowRequest {
  // URL of the repo the workflow is running for
  // Ex. "https://github.com/some-user/acme"
  string repo_url = 1;
  // Git refs at which the workflow should be run (at least one of `branch` or
  // `commit_sha` must be set). If only `branch` is set, will run from the tip
  // of the branch. If only `commit_sha` is set, reporting will not contain the
  // branch name.
  string branch = 8;
  string commit_sha = 9;

  // OPTIONAL FIELDS

  // Names of the workflow actions to execute. Correspond to actions
  // in buildbuddy.yaml, or actions in the default workflow config if
  // buildbuddy.yaml is missing from the repo.
  //
  // If this is not set, will execute all actions in the workflow config.
  //
  // Ex. ["Browser Tests", "Docker tests"]
  repeated string action_names = 3;
  // VISIBILITY build metadata used for the workflow invocation.
  // Workflow invocations are private by default, but this can be
  // set to "PUBLIC" to make the workflow invocation public.
  string visibility = 5;
  // If true, start the workflow but do not wait for the status to be returned.
  bool async = 6;
  // These env vars will be applied to each action that is run. If there is a
  // conflict between an env var set here and in buildbuddy.yaml, these
  // overrides will take precedence. Otherwise all env vars set in
  // buildbuddy.yaml will still apply.
  map<string, string> env = 7;
}
```

### ExecuteWorkflowResponse

```protobuf
message ExecuteWorkflowResponse {
  message ActionStatus {
    // Corresponds to an action in buildbuddy.yaml, or an action name in the
    // default workflow config if buildbuddy.yaml is missing from the repo. Ex.
    // "Test all targets"
    string action_name = 1;

    // The BuildBuddy invocation ID from executing the action.
    string invocation_id = 2;

    // The gRPC status from executing the action, or from starting the action if
    // async is true.
    google.rpc.Status status = 3;
  }

  // A list of the actions executed by the API, or actions started if async is
  // true.
  repeated ActionStatus action_statuses = 1;
}
```

## Run

The `Run` endpoint lets you run a command on a remote runner, which is hosted on
a remote executor.

### Endpoint

```
https://app.buildbuddy.io/api/v1/Run
```

### Service

```protobuf
rpc Run(RunRequest) returns (RunResponse);
```

### Example cURL request

```bash
curl -d '{
    "repo”: "git@github.com:buildbuddy-io/buildbuddy.git",
    "branch":"main",
    "steps": [{"run": "bazel test //..."}]
}' \
-H "x-buildbuddy-api-key: YOUR_BUILDBUDDY_API_KEY" \
-H 'Content-Type: application/json' \
https://app.buildbuddy.io/api/v1/Run
```

### RunRequest

```protobuf
message RunRequest {
  // URL of the repo the remote workspace should be initialized for
  // Ex. "https://github.com/some-user/acme"
  string repo = 1;

  // Git refs to configure the remote git workspace (at least one of `branch` or
  // `commit_sha` must be set). If only `branch` is set, will run from the tip
  // of the branch. If only `commit_sha` is set, reporting will not contain the
  // branch name.
  string branch = 2;
  string commit_sha = 3;

  // Bash commands to run, in order.
  // If a step fails, subsequent steps are not run.
  // Ex. [{"run": "bazel build :server"}, {"run": "echo \"Hello World\""}]
  repeated Step steps = 4;

  // Environment variables to set in the runner env.
  map<string, string> env = 5;

  // Platform properties to apply to the runner.
  // Ex. {"OSFamily":"linux", "Arch":"amd64"}
  map<string, string> platform_properties = 6;

  // Max time before run should be canceled.
  // Ex. "15s", "2h"
  string timeout = 7;

  // If true, start the runner but do not wait for the action to be scheduled
  // before returning a response.
  bool async = 8;

  // Whether to use github credentials configured on the executor.
  //
  // By default, we require https for git operations and generate short-lived
  // access tokens using our github app installation.
  // If GitHub SSH access is already configured on the runners, set this
  // to true to skip doing that and use the configured auth.
  //
  // This is only supported for bare runners.
  bool use_system_git_credentials = 9;
}

message Step {
  string run = 1;
}
```

### RunResponse

```protobuf
message RunResponse {
  // The invocation ID of the remote run.
  string invocation_id = 1;
}
```
