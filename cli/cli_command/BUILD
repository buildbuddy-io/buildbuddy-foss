load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cli_command",
    srcs = ["cli_command.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/cli/cli_command",
    deps = [
        "//cli/add",
        "//cli/analyze",
        "//cli/ask",
        "//cli/download",
        "//cli/execute",
        "//cli/explain",
        "//cli/fix",
        "//cli/index",
        "//cli/login",
        "//cli/plugin",
        "//cli/printlog",
        "//cli/remote_download",
        "//cli/remotebazel",
        "//cli/search",
        "//cli/update",
        "//cli/upload",
        "//cli/versioncmd",
    ],
)

package(default_visibility = ["//cli:__subpackages__"])
