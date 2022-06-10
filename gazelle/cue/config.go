package cuelang

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	gzflag "github.com/bazelbuild/bazel-gazelle/flag"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// cueConfig contains configuration values related to Cue rules
type cueConfig struct {
	// prefix is a prefix of an import path, used to generate importpath
	// attributes. Set with -cue_prefix or # gazelle:prefix.
	prefix string

	// prefixRel is the package name of the directory where the prefix was set
	// ("" for the root directory).
	prefixRel string

	// prefixSet indicates whether the prefix was set explicitly. It is an error
	// to infer an importpath for a rule without setting the prefix.
	prefixSet bool

	// importMapPrefix is a prefix of a package path, used to generate importmap
	// attributes. Set with # gazelle:importmap_prefix.
	importMapPrefix string

	// importMapPrefixRel is the package name of the directory where importMapPrefix
	// was set ("" for the root directory).
	importMapPrefixRel string

	// By default, internal packages are only visible to its siblings.
	// cueVisibility adds a list of packages the internal packages should be
	// visible to
	cueVisibility []string
}

func newCueConfig() *cueConfig {
	cc := &cueConfig{}
	return cc
}

func getCueConfig(c *config.Config) *cueConfig {
	return c.Exts[cueName].(*cueConfig)
}

func (cc *cueConfig) clone() *cueConfig {
	ccCopy := *cc
	return &ccCopy
}

func (*cueLang) KnownDirectives() []string {
	return []string{
		"cue_visibility",
		"importmap_prefix",
		"prefix",
	}
}

func (*cueLang) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	cc := newCueConfig()
	switch cmd {
	case "fix", "update":
		fs.Var(
			&gzflag.ExplicitFlag{Value: &cc.prefix, IsSet: &cc.prefixSet},
			"cue_prefix",
			"prefix of import paths in current workspace")
	}
	c.Exts[cueName] = cc
}

func (*cueLang) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

func (*cueLang) Configure(c *config.Config, rel string, f *rule.File) {
	var cc *cueConfig
	if raw, ok := c.Exts[cueName]; !ok {
		cc = newCueConfig()
	} else {
		cc = raw.(*cueConfig).clone()
	}
	c.Exts[cueName] = cc

	if f != nil {
		setPrefix := func(prefix string) {
			if err := checkPrefix(prefix); err != nil {
				log.Print(err)
				return
			}
			cc.prefix = prefix
			cc.prefixSet = true
			cc.prefixRel = rel
		}
		for _, d := range f.Directives {
			switch d.Key {
			case "cue_visibility":
				cc.cueVisibility = append(cc.cueVisibility, strings.TrimSpace(d.Value))
			case "importmap_prefix":
				cc.importMapPrefix = d.Value
				cc.importMapPrefixRel = rel
			case "prefix":
				setPrefix(d.Value)
			}
		}

		if !cc.prefixSet {
			for _, r := range f.Rules {
				switch r.Kind() {
				case "cue_prefix":
					args := r.Args()
					if len(args) != 1 {
						continue
					}
					s, ok := args[0].(*bzl.StringExpr)
					if !ok {
						continue
					}
					setPrefix(s.Value)
				case "gazelle":
					if prefix := r.AttrString("prefix"); prefix != "" {
						setPrefix(prefix)
					}
				}
			}
		}
	}
}

// checkPrefix checks that a string may be used as a prefix. We forbid local
// (relative) imports and those beginning with "/". We allow the empty string,
// but generated rules must not have an empty importpath.
func checkPrefix(prefix string) error {
	if strings.HasPrefix(prefix, "/") || build.IsLocalImport(prefix) {
		return fmt.Errorf("invalid prefix: %q", prefix)
	}
	return nil
}
