load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testdigest",
    srcs = ["testdigest.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/testutil/testdigest",
    visibility = ["//visibility:public"],
    deps = [
        "//proto:remote_execution_go_proto",
        "//proto:resource_go_proto",
        "//server/remote_cache/digest",
        "@com_github_stretchr_testify//require",
    ],
)
