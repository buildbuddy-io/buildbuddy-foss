load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "test_data",
    testonly = 1,
    srcs = ["test_data.go"],
    embedsrcs = [
        "bazel5.3.1_help_startup_options.txt",
        "bazel5.3.1_help_test.txt",
        "bazel5.3.1_help_query.txt",
        "bazel5.3.1_help_build.txt",
        "bazel5.3.1_help_run.txt",
    ],
    importpath = "github.com/buildbuddy-io/buildbuddy/cli/parser/test_data",
)

package(default_visibility = ["//cli:__subpackages__"])
