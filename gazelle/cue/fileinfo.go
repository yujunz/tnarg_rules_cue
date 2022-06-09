package cuelang

import (
	"log"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
)

// fileInfo holds information used to decide how to build a file. This
// information comes from the file's name, from package and import declarations
type fileInfo struct {
	path, name string

	// ext is the type of file, based on extension.
	ext ext

	// packageName is the Cue package name of a .cue file.
	// It is empty for non-Cue files.
	packageName string

	// imports is a list of packages imported by a file. It does not include
	// anything from the standard library.
	imports []string
}

// fileNameInfo returns information that can be inferred from the name of
// a file. It does not read data from the file.
func fileNameInfo(path_ string) fileInfo {
	name := filepath.Base(path_)
	var ext ext
	switch path.Ext(name) {
	case ".go":
		ext = cueExt
	default:
		ext = unknownExt
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		ext = unknownExt
	}
	return fileInfo{
		path: path_,
		name: name,
		ext:  ext,
	}
}

// cueFileInfo returns information about a .cue file. It will parse part of the
// file to determine the package name, imports, and build constraints.
// If the file can't be read, an error will be logged, and partial information
// will be returned.
func cueFileInfo(path, rel string) fileInfo {
	info := fileNameInfo(path)
	pf, err := parser.ParseFile(info.path, nil)
	if err != nil {
		log.Printf("%s: error reading cue file: %v", info.path, err)
		return info
	}

	info.packageName = pf.PackageName()

	for _, decl := range pf.Decls {
		d, ok := decl.(*ast.ImportDecl)
		if !ok {
			continue
		}
		for _, dspec := range d.Specs {
			quoted := dspec.Path.Value
			path, err := strconv.Unquote(quoted)
			if err != nil {
				log.Printf("%s: error reading cue file: %v", info.path, err)
				continue
			}
			info.imports = append(info.imports, path)
		}
	}
	return info
}

// otherFileInfo returns information about a non-.cue file.
func otherFileInfo(path string) fileInfo {
	return fileNameInfo(path)
}
