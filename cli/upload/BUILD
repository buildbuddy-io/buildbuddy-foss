load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "upload",
    srcs = ["upload.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/cli/upload",
    deps = [
        "//cli/arg",
        "//cli/log",
        "//cli/login",
        "//proto:remote_execution_go_proto",
        "//server/remote_cache/cachetools",
        "//server/remote_cache/digest",
        "//server/util/grpc_client",
        "@org_golang_google_genproto_googleapis_bytestream//:bytestream",
        "@org_golang_google_grpc//metadata",
    ],
)

package(default_visibility = ["//cli:__subpackages__"])
