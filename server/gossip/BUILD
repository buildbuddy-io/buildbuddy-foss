load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "gossip",
    srcs = ["gossip.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/gossip",
    visibility = ["//visibility:public"],
    deps = [
        "//server/hostid",
        "//server/interfaces",
        "//server/real_environment",
        "//server/util/flag",
        "//server/util/log",
        "//server/util/network",
        "//server/util/retry",
        "//server/util/status",
        "//server/util/statusz",
        "@com_github_hashicorp_memberlist//:memberlist",
        "@com_github_hashicorp_serf//serf",
        "@com_github_rs_zerolog//:zerolog",
    ],
)

go_test(
    name = "gossip_test",
    size = "small",
    srcs = ["gossip_test.go"],
    deps = [
        ":gossip",
        "//server/interfaces",
        "//server/testutil/quarantine",
        "//server/testutil/testport",
        "//server/util/testing/flags",
        "@com_github_hashicorp_serf//serf",
        "@com_github_stretchr_testify//require",
    ],
)
