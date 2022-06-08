/* Copyright 2018 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// cuelang provides a implementation of language.Language.
package cuelang

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const cueName = "cue"

type cueLang struct{}

func NewLanguage() language.Language {
	return &cueLang{}
}

func (*cueLang) Name() string {
	return cueName
}

func (*cueLang) Loads() []rule.LoadInfo {
	return nil
}

func (*cueLang) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
}

func (*cueLang) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

func (*cueLang) KnownDirectives() []string {
	return nil
}

func (*cueLang) Configure(c *config.Config, rel string, f *rule.File) {
}

func (*cueLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return language.GenerateResult{
		Gen:     []*rule.Rule{rule.NewRule("cue_library", "cue_default_library")},
		Imports: []interface{}{nil},
	}
}

func (*cueLang) Fix(c *config.Config, f *rule.File) {
}

func (*cueLang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	return nil
}

func (*cueLang) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return nil
}

func (*cueLang) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
}
