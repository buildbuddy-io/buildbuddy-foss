load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "terminal",
    srcs = ["terminal.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/cli/terminal",
    deps = ["@com_github_mattn_go_isatty//:go-isatty"],
)

package(default_visibility = ["//cli:__subpackages__"])
