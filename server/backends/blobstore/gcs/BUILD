load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gcs",
    srcs = ["gcs.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/server/backends/blobstore/gcs",
    visibility = ["//visibility:public"],
    deps = [
        "//server/backends/blobstore/util",
        "//server/interfaces",
        "//server/util/flag",
        "//server/util/ioutil",
        "//server/util/log",
        "//server/util/retry",
        "//server/util/status",
        "//server/util/tracing",
        "@com_google_cloud_go_storage//:storage",
        "@org_golang_google_api//googleapi",
        "@org_golang_google_api//option",
        "@org_golang_google_grpc//codes",
        "@org_golang_google_grpc//status",
    ],
)
