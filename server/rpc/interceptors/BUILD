load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "interceptors",
    srcs = ["interceptors.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/rpc/interceptors",
    visibility = ["//visibility:public"],
    deps = [
        "//server/capabilities_filter",
        "//server/environment",
        "//server/util/alert",
        "//server/util/authutil",
        "//server/util/bazel_request",
        "//server/util/claims",
        "//server/util/clientip",
        "//server/util/log",
        "//server/util/proto",
        "//server/util/request_context",
        "//server/util/status",
        "//server/util/subdomain",
        "//server/util/tracing",
        "//server/util/usageutil",
        "//server/util/uuid",
        "@com_github_grpc_ecosystem_go_grpc_middleware_providers_prometheus//:prometheus",
        "@com_github_prometheus_client_golang//prometheus",
        "@io_opentelemetry_go_otel//attribute",
        "@io_opentelemetry_go_otel_trace//:trace",
        "@org_golang_google_grpc//:grpc",
        "@org_golang_google_grpc//metadata",
        "@org_golang_google_grpc//peer",
    ],
)

go_test(
    name = "interceptors_test",
    size = "small",
    srcs = ["interceptors_test.go"],
    deps = [
        "//proto:ping_service_go_proto",
        "//server/testutil/testenv",
        "//server/testutil/testport",
        "//server/util/grpc_client",
        "//server/util/grpc_server",
        "//server/util/log",
        "//server/util/random",
        "@org_golang_google_grpc//:grpc",
    ],
)
