load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "grpc_client",
    srcs = ["grpc_client.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/util/grpc_client",
    visibility = ["//visibility:public"],
    deps = [
        "//server/environment",
        "//server/rpc/interceptors",
        "//server/util/canary",
        "//server/util/flag",
        "//server/util/log",
        "//server/util/status",
        "@org_golang_google_grpc//:grpc",
        "@org_golang_google_grpc//connectivity",
        "@org_golang_google_grpc//credentials/google",
        "@org_golang_google_grpc//experimental",
        "@org_golang_google_grpc//keepalive",
        "@org_golang_google_grpc//mem",
        "@org_golang_x_sync//errgroup",
    ],
)
