load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

# gazelle:default_visibility //cli:__subpackages__,//enterprise:__subpackages__
package(default_visibility = [
    "//cli:__subpackages__",
    "//enterprise:__subpackages__",
])

go_library(
    name = "remotebazel",
    srcs = ["remotebazel.go"],
    importpath = "github.com/buildbuddy-io/buildbuddy/cli/remotebazel",
    deps = [
        "//cli/arg",
        "//cli/log",
        "//cli/login",
        "//cli/parser",
        "//cli/storage",
        "//cli/terminal",
        "//proto:build_event_stream_go_proto",
        "//proto:buildbuddy_service_go_proto",
        "//proto:eventlog_go_proto",
        "//proto:execution_stats_go_proto",
        "//proto:git_go_proto",
        "//proto:invocation_go_proto",
        "//proto:remote_execution_go_proto",
        "//proto:runner_go_proto",
        "//proto/api/v1:common_go_proto",
        "//server/cache/dirtools",
        "//server/environment",
        "//server/real_environment",
        "//server/remote_cache/cachetools",
        "//server/remote_cache/digest",
        "//server/util/bazel",
        "//server/util/error_util",
        "//server/util/flag",
        "//server/util/grpc_client",
        "//server/util/rexec",
        "//server/util/shlex",
        "//server/util/status",
        "@com_github_alecaivazis_survey_v2//:survey",
        "@org_golang_google_genproto_googleapis_bytestream//:bytestream",
        "@org_golang_google_grpc//metadata",
        "@org_golang_x_sync//errgroup",
        "@org_golang_x_sys//unix",
    ],
)

go_test(
    name = "remotebazel_test",
    srcs = ["remotebazel_test.go"],
    embed = [":remotebazel"],
    deps = [
        "//cli/parser",
        "//cli/parser/test_data",
        "//server/testutil/testgit",
        "//server/testutil/testshell",
        "@com_github_stretchr_testify//require",
    ],
)
