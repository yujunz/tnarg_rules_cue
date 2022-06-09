package cuelang

import (
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
		// TODO(yujunz): addFile
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
