package cuelang

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/bazel-gazelle/walk"
)

func testConfig(t *testing.T, args ...string) (*config.Config, []language.Language, []config.Configurer) {
	// Add a -repo_root argument if none is present. Without this,
	// config.CommonConfigurer will try to auto-detect a WORKSPACE file,
	// which will fail.
	haveRoot := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-repo_root") {
			haveRoot = true
			break
		}
	}
	if !haveRoot {
		args = append(args, "-repo_root=.")
	}

	cexts := []config.Configurer{
		&config.CommonConfigurer{},
		&walk.Configurer{},
		&resolve.Configurer{},
	}
	langs := []language.Language{NewLanguage()}
	c := testtools.NewTestConfig(t, cexts, langs, args)
	for _, lang := range langs {
		cexts = append(cexts, lang)
	}
	return c, langs, cexts
}

func TestCommandLine(t *testing.T) {
	c, _, _ := testConfig(
		t,
		"-cue_prefix=example.com/repo",
		"-repo_root=.")
	cc := getCueConfig(c)
	if cc.prefix != "example.com/repo" {
		t.Errorf(`got prefix %q; want "example.com/repo"`, cc.prefix)
	}
}

func TestDirectives(t *testing.T) {
	c, _, cexts := testConfig(t)
	content := []byte(`
# gazelle:importmap_prefix x
# gazelle:prefix y
`)
	f, err := rule.LoadData(filepath.FromSlash("test/BUILD.bazel"), "test", content)
	if err != nil {
		t.Fatal(err)
	}
	for _, cext := range cexts {
		cext.Configure(c, "test", f)
	}
	cc := getCueConfig(c)
	if cc.prefix != "y" {
		t.Errorf(`got prefix %q; want "y"`, cc.prefix)
	}
	if cc.prefixRel != "test" {
		t.Errorf(`got prefixRel %q; want "test"`, cc.prefixRel)
	}
	if cc.importMapPrefix != "x" {
		t.Errorf(`got importMapPrefix %q; want "x"`, cc.importMapPrefix)
	}
	if cc.importMapPrefixRel != "test" {
		t.Errorf(`got importPrefixRel %q; want "test"`, cc.importMapPrefixRel)
	}
}

func TestPrefixFallback(t *testing.T) {
	c, _, cexts := testConfig(t)
	for _, tc := range []struct {
		desc, content, want string
	}{
		{
			desc: "cue_prefix",
			content: `
cue_prefix("example.com/repo")
`,
			want: "example.com/repo",
		}, {
			desc: "gazelle",
			content: `
gazelle(
    name = "gazelle",
    prefix = "example.com/repo",
)
`,
			want: "example.com/repo",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := rule.LoadData("BUILD.bazel", "", []byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			for _, cext := range cexts {
				cext.Configure(c, "x", f)
			}
			cc := getCueConfig(c)
			if !cc.prefixSet {
				t.Fatalf("prefix not set")
			}
			if cc.prefix != tc.want {
				t.Errorf("prefix: want %q; got %q", cc.prefix, tc.want)
			}
			if cc.prefixRel != "x" {
				t.Errorf("rel: got %q; want %q", cc.prefixRel, "x")
			}
		})
	}
}
