package cuelang

import "github.com/bazelbuild/bazel-gazelle/rule"

func (*cueLang) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"cue_instance": {
			NonEmptyAttrs: map[string]bool{"srcs": true},
		},
		"cue_module": {},
	}
}
