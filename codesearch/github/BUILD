load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "github",
    srcs = ["github.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/codesearch/github",
    visibility = ["//visibility:public"],
    deps = [
        "//codesearch/schema",
        "//server/util/git",
        "@com_github_cespare_xxhash_v2//:xxhash",
        "@com_github_gabriel_vasile_mimetype//:mimetype",
        "@com_github_go_enry_go_enry_v2//:go-enry",
    ],
)
