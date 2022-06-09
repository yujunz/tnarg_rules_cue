package cuelang

import (
	"log"
	"regexp"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
)

// cuePackage contains metadata for a set of .cue files that can be
// used to generate Cue rules.
type cuePackage struct {
	name, dir, rel string
	instance       cueTarget
	importPath     string
}

// addFile adds the file described by "info" to a target in the package "p" if
// the file is buildable.
//
// Files that are not buildable will not
// be added to any target (for example, .txt files).
func (pkg *cuePackage) addFile(c *config.Config, info fileInfo) error {
	switch {
	case info.ext == unknownExt:
		return nil
	default:
		pkg.instance.addFile(c, info)
	}

	return nil
}

func buildPackages(c *config.Config, dir, rel string, cueFiles []fileInfo) (packageMap map[string]*cuePackage, cueFilesWithUnkonwnPackage []fileInfo) {
	packageMap = make(map[string]*cuePackage)
	for _, f := range cueFiles {
		if f.packageName == "" {
			cueFilesWithUnkonwnPackage = append(cueFilesWithUnkonwnPackage, f)
			continue
		}

		if _, ok := packageMap[f.packageName]; !ok {
			packageMap[f.packageName] = &cuePackage{
				name: f.packageName,
				dir:  dir,
				rel:  rel,
			}
		}
		if err := packageMap[f.packageName].addFile(c, f); err != nil {
			log.Print(err)
		}
	}
	return packageMap, cueFilesWithUnkonwnPackage
}

// selectPackages selects one Cue packages found in a directory.
// If multiple packages are found, it returns the package
// whose name matches the directory if such a package exists.
func selectPackage(c *config.Config, dir string, packageMap map[string]*cuePackage) (*cuePackage, error) {
	if len(packageMap) == 1 {
		for _, pkg := range packageMap {
			return pkg, nil
		}
	}

	if pkg, ok := packageMap[defaultPackageName(c, dir)]; ok {
		return pkg, nil
	}

	// TODO(yujunz): handle errors of multiple packages without default
	return nil, nil
}

func defaultPackageName(c *config.Config, rel string) string {
	cc := getCueConfig(c)
	return pathtools.RelBaseName(rel, cc.prefix, "")
}

// namingConvention determines how go targets are named.
type namingConvention int

const (
	// Try to detect the naming convention in use.
	unknownNamingConvention namingConvention = iota

	// For an import path that ends with foo, the cue_instance rules target is
	// named 'foo'
	importNamingConvention
)

// Matches a package version, eg. the end segment of 'example.com/foo/v1'
var pkgVersionRe = regexp.MustCompile("^v[0-9]+$")

// instNameFromImportPath returns a a suitable cue_instance name based on the import path.
// Major version suffixes (eg. "v1") are dropped.
func instNameFromImportPath(dir string) string {
	i := strings.LastIndexAny(dir, "/\\")
	if i < 0 {
		return dir
	}
	name := dir[i+1:]
	if pkgVersionRe.MatchString(name) {
		dir := dir[:i]
		i = strings.LastIndexAny(dir, "/\\")
		if i >= 0 {
			name = dir[i+1:]
		}
	}
	return strings.ReplaceAll(name, ".", "_")
}

// instNameByConvention returns a suitable name for a cue_instance using the given
// naming convention, the import path, and the package name.
func instNameByConvention(nc namingConvention, imp string, pkgName string) string {
	name := instNameFromImportPath(imp)
	if name == "" {
		name = pkgName
	}
	return name
}
