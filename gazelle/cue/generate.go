package cuelang

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

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
func (cl *cueLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	c := args.Config

	regularFiles := append([]string{}, args.RegularFiles...)
	genFiles := append([]string{}, args.GenFiles...)

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

	g := &generator{
		c:                   c,
		rel:                 args.Rel,
		shouldSetVisibility: args.File == nil || !args.File.HasDefaultVisibility(),
	}
	var res language.GenerateResult
	var rules []*rule.Rule

	if pkg != nil {
		// Add files with unknown packages. This happens when there are parse
		// or I/O errors. We should keep the file in the srcs list and let the
		// compiler deal with the error.
		for _, info := range cueFilesWithUnkownPackage {
			if err := pkg.addFile(c, info); err != nil {
				log.Print(err)
			}
		}
		// Process the other static files.
		for _, file := range otherFiles {
			info := otherFileInfo(filepath.Join(args.Dir, file))
			if err := pkg.addFile(c, info); err != nil {
				log.Print(err)
			}
		}

		// Process generated files. Note that generated files may have the same names
		// as static files. Bazel will use the generated files, but we will look at
		// the content of static files, assuming they will be the same.
		regularFileSet := make(map[string]bool)
		for _, f := range regularFiles {
			regularFileSet[f] = true
		}
		// Some of the generated files may have been consumed by other rules
		consumedFileSet := make(map[string]bool)
		for _, r := range args.OtherGen {
			for _, f := range r.AttrStrings("srcs") {
				consumedFileSet[f] = true
			}
			if f := r.AttrString("src"); f != "" {
				consumedFileSet[f] = true
			}
		}
		for _, f := range genFiles {
			if regularFileSet[f] || consumedFileSet[f] {
				continue
			}
			info := fileNameInfo(filepath.Join(args.Dir, f))
			if err := pkg.addFile(c, info); err != nil {
				log.Print(err)
			}
		}

		inst := g.generateInst(pkg)
		rules = append(rules, inst)
	}

	for _, r := range rules {
		if r.IsEmpty(cueKinds[r.Kind()]) {
			res.Empty = append(res.Empty, r)
		} else {
			res.Gen = append(res.Gen, r)
			res.Imports = append(res.Imports, r.PrivateAttr(config.GazelleImportsKey))
		}
	}

	if args.File != nil || len(res.Gen) > 0 {
		cl.cuePkgRels[args.Rel] = true
	} else {
		for _, sub := range args.Subdirs {
			if cl.cuePkgRels[path.Join(args.Rel, sub)] {
				cl.cuePkgRels[args.Rel] = false
				break
			}
		}
	}

	return res
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

func (t *cueTarget) addFile(c *config.Config, info fileInfo) {
	add := func(sb *platformStringsBuilder, ss ...string) {
		for _, s := range ss {
			sb.addGenericString(s)
		}
	}
	add(&t.sources, info.name)
	add(&t.imports, info.imports...)
}

type generator struct {
	c                   *config.Config
	rel                 string
	shouldSetVisibility bool
}

func (g *generator) generateInst(pkg *cuePackage) *rule.Rule {
	cc := getCueConfig(g.c)
	name := instNameByConvention(cc.cueNamingConvention, pkg.importPath, pkg.name)
	cueInstance := rule.NewRule("cue_instance", name)
	if !pkg.instance.sources.hasCue() {
		return cueInstance // empty
	}
	visibility := g.commonVisibility(pkg.importPath)

	g.setCommonAttrs(cueInstance, pkg.rel, visibility, pkg.instance)
	g.setImportAttrs(cueInstance, pkg.importPath)
	return cueInstance
}

func (g *generator) commonVisibility(importPath string) []string {
	// If the Bazel package name (rel) contains "internal", add visibility for
	// subpackages of the parent.
	// If the import path contains "internal" but rel does not, this is
	// probably an internal submodule. Add visibility for all subpackages.
	relIndex := pathtools.Index(g.rel, "internal")
	importIndex := pathtools.Index(importPath, "internal")
	visibility := getCueConfig(g.c).cueVisibility
	if relIndex >= 0 {
		parent := strings.TrimSuffix(g.rel[:relIndex], "/")
		visibility = append(visibility, fmt.Sprintf("//%s:__subpackages__", parent))
	} else if importIndex >= 0 {
		// This entire module is within an internal directory.
		// Identify other repos which should have access too.
		visibility = append(visibility, "//:__subpackages__")
		for _, repo := range g.c.Repos {
			if pathtools.HasPrefix(repo.AttrString("importpath"), importPath[:importIndex]) {
				visibility = append(visibility, "@"+repo.Name()+"//:__subpackages__")
			}
		}

	} else {
		return []string{"//visibility:public"}
	}

	return visibility
}

func (g *generator) setCommonAttrs(r *rule.Rule, pkgRel string, visibility []string, target cueTarget) {
	if !target.sources.isEmpty() {
		r.SetAttr("srcs", target.sources.buildFlat())
	}
	if g.shouldSetVisibility && len(visibility) > 0 {
		r.SetAttr("visibility", visibility)
	}
	r.SetPrivateAttr(config.GazelleImportsKey, target.imports.build())
}

func (sb *platformStringsBuilder) addGenericString(s string) {
	if sb.strs == nil {
		sb.strs = make(map[string]platformStringInfo)
	}
	sb.strs[s] = platformStringInfo{set: genericSet}
}

func (g *generator) setImportAttrs(r *rule.Rule, importPath string) {
	cc := getCueConfig(g.c)
	r.SetAttr("importpath", importPath)

	if cc.importMapPrefix != "" {
		fromPrefixRel := pathtools.TrimPrefix(g.rel, cc.importMapPrefixRel)
		importMap := path.Join(cc.importMapPrefix, fromPrefixRel)
		if importMap != importPath {
			r.SetAttr("importmap", importMap)
		}
	}
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

const (
	genericSet platformStringSet = iota
	osSet
	archSet
	platformSet
)

// platformStringsBuilder is used to construct rule.PlatformStrings. Bazel
// has some requirements for deps list (a dependency cannot appear in more
// than one select expression; dependencies cannot be duplicated), so we need
// to build these carefully.
type platformStringsBuilder struct {
	strs map[string]platformStringInfo
}

func (sb *platformStringsBuilder) isEmpty() bool {
	return sb.strs == nil
}

func (sb *platformStringsBuilder) hasCue() bool {
	for s := range sb.strs {
		if strings.HasSuffix(s, ".cue") {
			return true
		}
	}
	return false
}

func (sb *platformStringsBuilder) build() rule.PlatformStrings {
	var ps rule.PlatformStrings
	for s, si := range sb.strs {
		switch si.set {
		case genericSet:
			ps.Generic = append(ps.Generic, s)
		case osSet:
			if ps.OS == nil {
				ps.OS = make(map[string][]string)
			}
			for os := range si.oss {
				ps.OS[os] = append(ps.OS[os], s)
			}
		case archSet:
			if ps.Arch == nil {
				ps.Arch = make(map[string][]string)
			}
			for arch := range si.archs {
				ps.Arch[arch] = append(ps.Arch[arch], s)
			}
		case platformSet:
			if ps.Platform == nil {
				ps.Platform = make(map[rule.Platform][]string)
			}
			for p := range si.platforms {
				ps.Platform[p] = append(ps.Platform[p], s)
			}
		}
	}
	sort.Strings(ps.Generic)
	if ps.OS != nil {
		for _, ss := range ps.OS {
			sort.Strings(ss)
		}
	}
	if ps.Arch != nil {
		for _, ss := range ps.Arch {
			sort.Strings(ss)
		}
	}
	if ps.Platform != nil {
		for _, ss := range ps.Platform {
			sort.Strings(ss)
		}
	}
	return ps
}

func (sb *platformStringsBuilder) buildFlat() []string {
	strs := make([]string, 0, len(sb.strs))
	for s := range sb.strs {
		strs = append(strs, s)
	}
	sort.Strings(strs)
	return strs
}
