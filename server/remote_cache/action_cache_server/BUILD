load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "action_cache_server",
    srcs = ["action_cache_server.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/remote_cache/action_cache_server",
    visibility = ["//visibility:public"],
    deps = [
        "//proto:capability_go_proto",
        "//proto:remote_execution_go_proto",
        "//proto:resource_go_proto",
        "//server/environment",
        "//server/hostid",
        "//server/interfaces",
        "//server/metrics",
        "//server/real_environment",
        "//server/remote_cache/digest",
        "//server/util/bazel_request",
        "//server/util/capabilities",
        "//server/util/flag",
        "//server/util/log",
        "//server/util/prefix",
        "//server/util/proto",
        "//server/util/status",
        "@com_github_prometheus_client_golang//prometheus",
        "@org_golang_x_sync//errgroup",
    ],
)

go_test(
    name = "action_cache_server_test",
    srcs = ["action_cache_server_test.go"],
    deps = [
        ":action_cache_server",
        "//proto:cache_go_proto",
        "//proto:remote_execution_go_proto",
        "//proto:resource_go_proto",
        "//server/backends/memory_metrics_collector",
        "//server/interfaces",
        "//server/metrics",
        "//server/remote_cache/byte_stream_server",
        "//server/remote_cache/cachetools",
        "//server/remote_cache/content_addressable_storage_server",
        "//server/remote_cache/digest",
        "//server/remote_cache/hit_tracker",
        "//server/testutil/testenv",
        "//server/testutil/testmetrics",
        "//server/util/bazel_request",
        "//server/util/proto",
        "//server/util/status",
        "//server/util/testing/flags",
        "@com_github_google_go_cmp//cmp",
        "@com_github_prometheus_client_golang//prometheus",
        "@com_github_prometheus_client_golang//prometheus/testutil",
        "@com_github_stretchr_testify//assert",
        "@com_github_stretchr_testify//require",
        "@org_golang_google_genproto_googleapis_bytestream//:bytestream",
        "@org_golang_google_genproto_googleapis_rpc//status",
        "@org_golang_google_grpc//:grpc",
        "@org_golang_google_grpc//codes",
        "@org_golang_google_protobuf//testing/protocmp",
        "@org_golang_google_protobuf//types/known/timestamppb",
    ],
)
