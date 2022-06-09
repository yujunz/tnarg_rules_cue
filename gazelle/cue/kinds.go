package cuelang

import "github.com/bazelbuild/bazel-gazelle/rule"

// Kinds returns a map of maps rule names (kinds) and information on how to
// match and merge attributes that may be found in rules of those kinds. All
// kinds of rules generated for this language may be found here.
func (*cueLang) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"cue_module": {
			MatchAttrs:    []string{"file"},
			NonEmptyAttrs: map[string]bool{"srcs": true},
		},

		"cue_instance": {
			MatchAttrs:    []string{"directory_of", "package_name"},
			NonEmptyAttrs: map[string]bool{"srcs": true},
		},
	}
}

// Loads returns .bzl files and symbols they define. Every rule generated by
// GenerateRules, now or in the past, should be loadable from one of these
// files.
func (*cueLang) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{
		{
			Name: "@com_github_yujunz_rules_cue//cue:deps.bzl",
			Symbols: []string{
				"cue_rules_dependencies",
				"cue_register_tool",
			},
		}, {
			Name: "@com_github_yujunz_rules_cue//cue:cue.bzl",
			Symbols: []string{
				"cue_instance",
				"cue_module",
			},
		},
	}
}