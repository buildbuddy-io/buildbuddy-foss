load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "replay_invocation_lib",
    srcs = ["replay_invocation.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/tools/replay_invocation",
    visibility = ["//visibility:private"],
    deps = [
        "//proto:build_event_stream_go_proto",
        "//proto:build_events_go_proto",
        "//proto:invocation_go_proto",
        "//proto:publish_build_event_go_proto",
        "//server/backends/blobstore",
        "//server/backends/chunkstore",
        "//server/build_event_protocol/accumulator",
        "//server/build_event_protocol/build_event_handler",
        "//server/eventlog",
        "//server/interfaces",
        "//server/real_environment",
        "//server/remote_cache/cachetools",
        "//server/remote_cache/digest",
        "//server/util/grpc_client",
        "//server/util/healthcheck",
        "//server/util/log",
        "//server/util/proto",
        "//server/util/protofile",
        "//server/util/status",
        "@com_github_google_uuid//:uuid",
        "@org_golang_google_genproto_googleapis_bytestream//:bytestream",
        "@org_golang_google_grpc//metadata",
        "@org_golang_google_protobuf//encoding/protojson",
        "@org_golang_google_protobuf//types/known/anypb",
        "@org_golang_google_protobuf//types/known/timestamppb",
    ],
)

go_binary(
    name = "replay_invocation",
    embed = [":replay_invocation_lib"],
    visibility = ["//visibility:public"],
)
