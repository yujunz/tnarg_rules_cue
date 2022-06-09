package cuelang

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type generator struct {
	c                   *config.Config
	rel                 string
	shouldSetVisibility bool
}

// GenerateRules extracts build metadata from source files in a directory.
// GenerateRules is called in each directory where an update is requested
// in depth-first post-order.
//
// args contains the arguments for GenerateRules. This is passed as a
// struct to avoid breaking implementations in the future when new
// fields are added.
//
// A GenerateResult struct is returned. Optional fields may be added to this
// type in the future.
//
// Any non-fatal errors this function encounters should be logged using
// log.Print.
func (*cueLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	c := args.Config
	regularFiles := append([]string{}, args.RegularFiles...)

	var cueFiles, otherFiles []string
	for _, f := range regularFiles {
		if strings.HasSuffix(f, ".cue") {
			cueFiles = append(cueFiles, f)
		} else {
			otherFiles = append(otherFiles, f)
		}
	}

	cueFileInfos := make([]fileInfo, len(cueFiles))
	for i, name := range cueFiles {
		path := filepath.Join(args.Dir, name)
		cueFileInfos[i] = cueFileInfo(path, args.Rel)
	}
	cuePackageMap, cueFilesWithUnkownPackage := buildPackages(c, args.Dir, args.Rel, cueFileInfos)
	pkg, err := selectPackage(c, args.Dir, cuePackageMap)
	if err != nil {
		log.Print(err)
	}

	if pkg != nil {
		for _, info := range cueFilesWithUnkownPackage {
			if err := pkg.addFile(c, info); err != nil {
				log.Print(err)
			}
		}
	}

	return language.GenerateResult{
		Gen:     []*rule.Rule{rule.NewRule("cue_library", "cue_default_library")},
		Imports: []interface{}{nil},
	}
}

type ext int

const (
	// unknownExt is applied files that aren't buildable with Go.
	unknownExt ext = iota

	// cueExt is applied to .cue files.
	cueExt
)

// cueTarget contains information used to generate an individual Cue rule
// (insatnce, or module).
type cueTarget struct {
	sources, imports platformStringsBuilder
}

// platformStringsBuilder is used to construct rule.PlatformStrings. Bazel
// has some requirements for deps list (a dependency cannot appear in more
// than one select expression; dependencies cannot be duplicated), so we need
// to build these carefully.
type platformStringsBuilder struct {
	strs map[string]platformStringInfo
}

const (
	genericSet platformStringSet = iota
)

func (sb *platformStringsBuilder) addGenericString(s string) {
	if sb.strs == nil {
		sb.strs = make(map[string]platformStringInfo)
	}
	sb.strs[s] = platformStringInfo{set: genericSet}
}

// platformStringInfo contains information about a single string (source,
// import, or option).
type platformStringInfo struct {
	set       platformStringSet
	oss       map[string]bool
	archs     map[string]bool
	platforms map[rule.Platform]bool
}

type platformStringSet int

func (t *cueTarget) addFile(c *config.Config, info fileInfo) {
	add := func(sb *platformStringsBuilder, ss ...string) {
		for _, s := range ss {
			sb.addGenericString(s)
		}
	}
	add(&t.sources, info.name)
	add(&t.imports, info.imports...)
}

func getCueConfig(c *config.Config) *cueConfig {
	return c.Exts[cueName].(*cueConfig)
}

// cueConfig contains configuration values related to Cue rules
type cueConfig struct {
	// prefix is a prefix of an import path, used to generate importpath
	// attributes. Set with -cue_prefix or # gazelle:prefix.
	prefix string
}
