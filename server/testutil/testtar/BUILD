load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testtar",
    testonly = 1,
    srcs = ["testtar.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/testutil/testtar",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_stretchr_testify//require",
    ],
)
