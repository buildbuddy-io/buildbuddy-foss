load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "usageutil",
    srcs = ["usageutil.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/util/usageutil",
    visibility = ["//visibility:public"],
    deps = [
        "//server/interfaces",
        "//server/tables",
        "//server/util/bazel_request",
        "//server/util/claims",
        "@org_golang_google_grpc//metadata",
    ],
)

go_test(
    name = "usageutil_test",
    srcs = ["usageutil_test.go"],
    deps = [
        ":usageutil",
        "//proto:remote_execution_go_proto",
        "//server/tables",
        "//server/util/bazel_request",
        "//server/util/proto",
        "//server/util/testing/flags",
        "@com_github_stretchr_testify//require",
        "@org_golang_google_grpc//metadata",
    ],
)
