load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "accumulator",
    srcs = ["accumulator.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/build_event_protocol/accumulator",
    visibility = ["//visibility:public"],
    deps = [
        "//proto:build_event_stream_go_proto",
        "//proto:invocation_go_proto",
        "//proto:invocation_status_go_proto",
        "//proto:resource_go_proto",
        "//server/build_event_protocol/event_parser",
        "//server/build_event_protocol/invocation_format",
        "//server/remote_cache/digest",
        "//server/util/log",
        "//server/util/proto",
        "//server/util/timeutil",
    ],
)
