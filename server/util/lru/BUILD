load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "lru",
    srcs = ["lru.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/util/lru",
    visibility = ["//visibility:public"],
    deps = ["//server/util/status"],
)

go_test(
    name = "lru_test",
    size = "small",
    srcs = ["lru_test.go"],
    deps = [
        ":lru",
        "@com_github_stretchr_testify//require",
    ],
)
