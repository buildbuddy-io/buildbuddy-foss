load("//rules/typescript:index.bzl", "ts_library")

package(default_visibility = ["//visibility:public"])

ts_library(
    name = "capabilities",
    srcs = ["capabilities.tsx"],
    deps = [
        "//:node_modules/tslib",
        "//proto:config_ts_proto",
    ],
)
