load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testnetworking",
    testonly = 1,
    srcs = ["testnetworking.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/testutil/testnetworking",
    visibility = ["//visibility:public"],
    deps = [
        "//server/util/lockingbuffer",
        "//server/util/log",
        "//server/util/networking",
        "@com_github_stretchr_testify//require",
    ],
)
