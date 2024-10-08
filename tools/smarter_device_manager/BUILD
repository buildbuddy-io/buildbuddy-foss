load("@io_bazel_rules_docker//container:container.bzl", "container_push")
load("@io_bazel_rules_docker//go:image.bzl", "go_image")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_push")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

# We use this internally so create an alias here to
# ensure our changes don't break the internal build.
alias(
    name = "smarter-device-manager",
    actual = "@com_gitlab_arm_research_smarter_smarter_device_manager//:smarter-device-manager",
)

go_image(
    name = "smarter_device_manager_image",
    base = "@buildbuddy_go_image_base//image",
    binary = ":smarter-device-manager",
    tags = ["manual"],
    visibility = ["//visibility:public"],
)

container_push(
    name = "push_image",
    format = "Docker",
    image = ":smarter_device_manager_image",
    registry = "gcr.io",
    repository = "flame-build/smarter-device-manager",
    tag = "server_image",
    tags = [
        "manual",
    ],
)

pkg_tar(
    name = "tar",
    srcs = [":smarter-device-manager"],
)

oci_image(
    name = "oci_image",
    base = "@buildbuddy_go_oci_image_base",
    entrypoint = ["/smarter-device-manager"],
    target_compatible_with = ["@platforms//os:linux"],
    tars = [":tar"],
    visibility = ["//visibility:public"],
)

oci_push(
    name = "push",
    image = ":oci_image",
    # Set the image tag with the bazel run flag "--//deployment:image_tag=TAG"
    remote_tags = "//deployment:oci_tag_file",
    repository = "gcr.io/flame-build/smarter-device-manager-oci",
)
