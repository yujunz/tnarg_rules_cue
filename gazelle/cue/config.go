package cuelang

import (
	"github.com/bazelbuild/bazel-gazelle/config"
)

func getCueConfig(c *config.Config) *cueConfig {
	return c.Exts[cueName].(*cueConfig)
}

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

	// cueNamingConvention controls the name of generated targets
	cueNamingConvention namingConvention

	// By default, internal packages are only visible to its siblings.
	// cueVisibility adds a list of packages the internal packages should be
	// visible to
	cueVisibility []string
}
