load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testauth",
    testonly = 1,
    srcs = ["testauth.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/testutil/testauth",
    visibility = ["//visibility:public"],
    deps = [
        "//proto:context_go_proto",
        "//proto:user_id_go_proto",
        "//server/interfaces",
        "//server/nullauth",
        "//server/tables",
        "//server/util/authutil",
        "//server/util/capabilities",
        "//server/util/claims",
        "//server/util/log",
        "//server/util/role",
        "//server/util/status",
        "@org_golang_google_grpc//metadata",
    ],
)
