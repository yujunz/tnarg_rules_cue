load(
    "@io_bazel_rules_go//go/private:common.bzl",
    "env_execute",
)

CUEStandaloneInstanceInfo = provider(
    doc = "Collects files from cue_library targets for use in downstream cue_export targets.",
    fields = {
        "def_file": """The output of the "cue def" tool summarizing this package as captured.""",
        # TODO(seh): Consider identifying the package name and instance name (if different).
        "import_path": "The path by which consuming packages import this package.",
        "transitive_instances": "The set of instances referenced by this instance.",
    },
)

def _add_common_attrs_to(attrs):
    attrs.update({
        "_cue": attr.label(
            default = "//cue:cue_runtime",
            executable = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "_cue_config": attr.label(
            default = "//:cue_config",
        ),
        "_replacer": attr.label(
            default = _replacer_if_stamping,
            executable = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "srcs": attr.label_list(
            doc = "Input files.",
            mandatory = True,
            allow_empty = False,
            allow_files = True,
        ),
        "deps": attr.label_list(
            doc = "cue_library targets to include in the evaluation",
            providers = [CUEStandaloneInstanceInfo],
            allow_files = False,
        ),
        "expression": attr.string(
            doc = "CUE expression selecting a single value to export.",
            default = "",
        ),
        "inject": attr.string_dict(
            doc = "Keys and values of tagged fields.",
        ),
        "inject_shorthand": attr.string_list(
            doc = "Shorthand values of tagged fields.",
        ),
        "concatenate_objects": attr.bool(
            doc = "Concatenate multiple objects into a list.",
        ),
        "merge_other_files": attr.bool(
            doc = "Merge non-CUE files.",
            default = True,
        ),
        "path": attr.string_list(
            doc = """Elements of CUE path at which to place top-level values.
Each entry for an element may nominate either a CUE field, ending with
either ":" for a regular fiield or "::" for a definition, or a CUE
expression, both variants evaluated within the value, unless
"with_context" is true.""",
        ),
        "stamping_policy": attr.string(
            doc = """Whether to stamp tagged field values before injection.

If "Allow," stamp tagged field values only when the "--stamp" flag is
active.

If "Force," stamp tagged field values unconditionally, even if the
"--stamp" flag is inactive.

If "Prevent," never stamp tagged field values, even if the "--stamp"
flag is active.""",
            values = [
                "Allow",
                "Force",
                "Prevent",
            ],
            default = "Allow",
        ),
        "with_context": attr.bool(
            doc = """Evaluate "path" elements within a struct of contextual data.
Instead of evaluating these elements in the context of the value being
situated, instead evaluate them within a struct identifying the source
data, file name, record index, and record count.""",
        ),
    })
    return attrs

def _add_common_args(ctx, args, stamped_args_file):
    cue_config = ctx.attr._cue_config[CUEConfigInfo]
    stamping_enabled = ctx.attr.stamping_policy == "Force" or ctx.attr.stamping_policy == "Allow" and cue_config.stamp
    required_stamp_bindings = {}

    # TODO(seh): Consider these:
    #if ctx.attr.simplify:
    #    args.add("--simplify")
    if ctx.attr.expression:
        args.add("--expression", ctx.attr.expression)
    for k, v in ctx.attr.inject.items():
        if len(k) == 0:
            fail(msg = "injected key must not empty")
        if stamping_enabled and v.startswith("{") and v.endswith("}"):
            required_stamp_bindings[k] = v[1:-1]
            continue
        args.add(
            "--inject",
            # Allow the empty string as a specified value.
            "{}={}".format(k, v),
        )
    for v in ctx.attr.inject_shorthand:
        if len(v) == 0:
            fail(msg = "injected value must not empty")
        args.add("--inject", v)
    if ctx.attr.concatenate_objects:
        args.add("--list")
    if not ctx.attr.merge_other_files:
        args.add("--merge=false")
    for p in ctx.attr.path:
        if not p:
            fail(msg = "path element must not be empty")
        args.add("--path", p)
    if ctx.attr.with_context:
        args.add("--with-context")
    args.add_all(ctx.files.srcs)

    if len(required_stamp_bindings) == 0:
        # Create an empty file, in order to unify the command that
        # consumes extra arguments when stamping is in effect.
        ctx.actions.write(
            stamped_args_file,
            "",
        )
    else:
        stamp_placeholders_file = ctx.actions.declare_file("{}-stamp-bindings".format(ctx.label.name))
        ctx.actions.write(
            stamp_placeholders_file,
            "\n".join([k + "=" + v for k, v in required_stamp_bindings.items()]),
        )
        args = ctx.actions.args()
        args.add("-prefix", "--inject ")
        args.add("-output", stamped_args_file.path)
        args.add(stamp_placeholders_file.path)
        args.add(ctx.info_file.path)  # stable-status.txt
        args.add(ctx.version_file.path)  # volatile-status.txt
        ctx.actions.run(
            executable = ctx.executable._replacer,
            arguments = [args],
            inputs = [
                stamp_placeholders_file,
                ctx.info_file,
                ctx.version_file,
            ],
            outputs = [stamped_args_file],
            mnemonic = "CUEReplaceStampBindings",
            progress_message = "Replacing injection placeholders with stamped values for {}".format(ctx.label.name),
        )

def _make_cue_mod_init_action(ctx):
    cue_module_file = ctx.actions.declare_file("cue.mod/module.cue")

    # NB: The "cue mod init" command insists that the "cue.mod"
    # directory must not exist beforehand, but Bazel will always
    # create a declared directory or the directory containing a
    # declared file before running an action that uses it.
    ctx.actions.write(
        cue_module_file,
        """\
module: ""
""",
    )
    return cue_module_file

def _accumulate_cue_library(ctx, lib, files, links):
    info = lib[CUEStandaloneInstanceInfo]

    cue_def_file = info.def_file
    files.append(cue_def_file)

    location_id = info.import_path.split(":", 1)[0]
    module_def_link = ctx.actions.declare_file(
        "/".join(["cue.mod/pkg", location_id, cue_def_file.basename]),
    )
    links.append(module_def_link)

    ctx.actions.symlink(
        output = module_def_link,
        target_file = cue_def_file,
    )

def _set_up_cue_module(ctx, cue_module_file, libraries):
    files = [cue_module_file]
    links = []
    for lib in libraries:
        _accumulate_cue_library(ctx, lib, files, links)
        for lib in lib[CUEStandaloneInstanceInfo].transitive_instances.to_list():
            _accumulate_cue_library(ctx, lib, files, links)

    return files + links

def _make_cue_def_action(ctx, cue_module_files):
    def_output_file = ctx.actions.declare_file(ctx.label.name + "~def.cue")

    args = ctx.actions.args()
    args.add(ctx.executable._cue.path)
    stamped_args_file = ctx.actions.declare_file("%s-stamped-args" % ctx.label.name)
    args.add(stamped_args_file.path)
    args.add(def_output_file.path)
    _add_common_args(ctx, args, stamped_args_file)

    ctx.actions.run_shell(
        mnemonic = "CUEDef",
        tools = [ctx.executable._cue],
        arguments = [args],
        command = """\
set -euo pipefail

cue=$1; shift
extra_args=$1; shift
output_file=$1; shift

ln -s "$(dirname ${output_file})/cue.mod" ./cue.mod
"${cue}" def --outfile "${output_file}" $(< "${extra_args}") "${@}"
""",
        inputs = cue_module_files +
                 ctx.files.srcs +
                 [
                     stamped_args_file,
                 ],
        outputs = [
            def_output_file,
        ],
    )

    return def_output_file

def _cue_library_impl(ctx):
    """cue_library_impl validates and summarizes a CUE package.

    It uses the "cue def" command to summarize the files into a single
    CUE file, and collects all transitive dependencies' information for
    reconstructing them in downstream evaluations.

    Args:
      ctx: The Bazel build context
    Returns:
      The cue_library rule.
    """
    cue_module_file = _make_cue_mod_init_action(ctx)
    cue_module_files = _set_up_cue_module(ctx, cue_module_file, ctx.attr.deps)
    def_output_file = _make_cue_def_action(ctx, cue_module_files)

    return [
        CUEStandaloneInstanceInfo(
            def_file = def_output_file,
            import_path = ctx.attr.import_path,
            transitive_instances = depset(
                direct = ctx.attr.deps,
                transitive = [dep[CUEStandaloneInstanceInfo].transitive_instances for dep in ctx.attr.deps],
                # Provide CUE sources from dependencies first.
                order = "postorder",
            ),
        ),
    ]

_cue_library_attrs = _add_common_attrs_to({
    "import_path": attr.string(
        doc = "CUE import path for this package.",
        mandatory = True,
    ),
})

cue_library = rule(
    implementation = _cue_library_impl,
    attrs = _cue_library_attrs,
)

def _cue_export_impl(ctx):
    """_cue_export_impl performs an action to export a set of input files."""

    cue_mod_directory = _make_cue_mod_init_action(ctx)
    cue_module_files = _set_up_cue_module(ctx, cue_mod_directory, ctx.attr.deps)
    output = ctx.outputs.export

    args = ctx.actions.args()
    args.add(ctx.executable._cue.path)
    stamped_args_file = ctx.actions.declare_file("%s-stamped-args" % ctx.label.name)
    args.add(stamped_args_file.path)
    args.add(output.path)

    if ctx.attr.escape:
        args.add("--escape")
    args.add("--out=" + ctx.attr.output_format)
    cue_mod_dir = _add_common_args(ctx, args, stamped_args_file)

    ctx.actions.run_shell(
        mnemonic = "CUEExport",
        tools = [ctx.executable._cue],
        arguments = [args],
        command = """
set -euo pipefail

cue=$1; shift
extra_args=$1; shift
output_file=$1; shift

ln -s "$(dirname ${output_file})/cue.mod" ./cue.mod
${cue} export --outfile "${output_file}" $(< "${extra_args}") "${@}"
""",
        inputs = ctx.files.srcs +
                 cue_module_files + [
            stamped_args_file,
        ],
        outputs = [
            output,
        ],
    )

def _strip_extension(path):
    """Removes the final extension from a path."""
    return path.rsplit(".", 1)[0]

def _cue_export_outputs(srcs, output_name, output_format):
    """Get map of cue_export outputs.
    Note that the arguments to this function are named after attributes on the rule.
    Args:
      srcs: The rule's `srcs` attribute
      output_name: The rule's `output_name` attribute
      output_format: The rule's `output_format` attribute
    Returns:
      Outputs for the cue_export
    """
    extension_by_format = {
        "json": "json",
        "text": "txt",
        "yaml": "yaml",
    }
    outputs = {
        "export": output_name or _strip_extension(srcs[0].name) + "." + extension_by_format[output_format],
    }

    return outputs

_cue_export_attrs = _add_common_attrs_to({
    "escape": attr.bool(
        doc = "Use HTML escaping.",
        default = False,
    ),

    #debug            give detailed error info
    #ignore           proceed in the presence of errors
    #simplify         simplify output
    #trace            trace computation
    #verbose          print information about progress
    "output_name": attr.string(
        doc = """Name of the output file, including the extension.
By default, this is based on the first entry in the `srcs` attribute:
if `foo.cue` is the first value in `srcs` then the output file is
`foo.json.`.  You can override this to be any other name.  Note that
some tooling may assume that the output name is derived from the input
name, so use this attribute with caution.""",
    ),
    "output_format": attr.string(
        doc = "Output format",
        default = "json",
        values = [
            "json",
            "text",
            "yaml",
        ],
    ),
})

cue_export = rule(
    implementation = _cue_export_impl,
    attrs = _cue_export_attrs,
    outputs = _cue_export_outputs,
)

# Copied from @bazel_tools//tools/build_defs/repo:utils.bzl
def _patch(ctx):
    """Implementation of patching an already extracted repository"""
    bash_exe = ctx.os.environ["BAZEL_SH"] if "BAZEL_SH" in ctx.os.environ else "bash"
    for patchfile in ctx.attr.patches:
        command = "{patchtool} {patch_args} < {patchfile}".format(
            patchtool = ctx.attr.patch_tool,
            patchfile = ctx.path(patchfile),
            patch_args = " ".join([
                "'%s'" % arg
                for arg in ctx.attr.patch_args
            ]),
        )
        st = ctx.execute([bash_exe, "-c", command])
        if st.return_code:
            fail("Error applying patch %s:\n%s%s" %
                 (str(patchfile), st.stderr, st.stdout))
    for cmd in ctx.attr.patch_cmds:
        st = ctx.execute([bash_exe, "-c", cmd])
        if st.return_code:
            fail("Error applying patch command %s:\n%s%s" %
                 (cmd, st.stdout, st.stderr))

# We can't disable timeouts on Bazel, but we can set them to large values.
_CUE_REPOSITORY_TIMEOUT = 86400

def _cue_repository_impl(ctx):
    # Download the repository archive
    ctx.download_and_extract(
        url = ctx.attr.urls,
        sha256 = ctx.attr.sha256,
        stripPrefix = ctx.attr.strip_prefix,
        type = ctx.attr.type,
    )

    # Repository is fetched. Determine if build file generation is needed.
    build_file_names = ctx.attr.build_file_name.split(",")
    existing_build_file = ""
    for name in build_file_names:
        path = ctx.path(name)
        if path.exists and not env_execute(ctx, ["test", "-f", path]).return_code:
            existing_build_file = name
            break

    generate = (ctx.attr.build_file_generation == "on" or (not existing_build_file and ctx.attr.build_file_generation == "auto"))

    if generate:
        # Build file generation is needed. Populate Gazelle directive at root build file
        build_file_name = existing_build_file or build_file_names[0]
        if len(ctx.attr.build_directives) > 0:
            ctx.file(
                build_file_name,
                "\n".join(["# " + d for d in ctx.attr.build_directives]),
            )

        # Run Gazelle
        _gazelle = "@com_github_tnarg_rules_cue//:gazelle_binary"
        gazelle = ctx.path(Label(_gazelle))
        cmd = [
            gazelle,
            "-cue_repository_mode",
            "-cue_prefix",
            ctx.attr.import_path,
            "-mode",
            "fix",
            "-repo_root",
            ctx.path(""),
        ]
        if ctx.attr.build_config:
            cmd.extend(["-repo_config"], ctx.path(ctx.attr.build_config))
        if ctx.attr.build_file_name:
            cmd.extend(["-build_file_name", ctx.attr.build_file_name])
        cmd.extend(ctx.attr.build_extra_args)
        cmd.append(ctx.path(""))
        result = env_execute(ctx, cmd, timeout = _CUE_REPOSITORY_TIMEOUT)
        if result.return_code:
            fail("failed to generate BUILD files for %s: %s" % (
                ctx.attr.import_path,
                result.stderr,
            ))
        if result.stderr:
            print("%s: %s" % (ctx.name, result.stderr))

    _patch(ctx)

cue_repository = repository_rule(
    implementation = _cue_repository_impl,
    attrs = {
        # Fundamental attributes of a CUE repository.
        "import_path": attr.string(mandatory = True),

        # Attributes for a repository that should be downloaded via HTTP.
        "urls": attr.string_list(),
        "strip_prefix": attr.string(),
        "type": attr.string(),
        "sha256": attr.string(),

        # Attributes for a repository that needs automatic BUILD file generation.
        "build_file_name": attr.string(default = "BUILD.bazel,BUILD"),
        "build_file_generation": attr.string(
            default = "auto",
            values = [
                "on",
                "auto",
                "off",
            ],
        ),
        "build_extra_args": attr.string_list(),
        "build_config": attr.label(),
        "build_directives": attr.string_list(default = []),

        # Patches to apply after running gazelle.
        "patches": attr.label_list(),
        "patch_tool": attr.string(default = "patch"),
        "patch_args": attr.string_list(default = ["-p0"]),
        "patch_cmds": attr.string_list(default = []),
    },
)