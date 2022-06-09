package cuelang

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
	return fileInfo{}
}

// cueFileInfo returns information about a .cue file. It will parse part of the
// file to determine the package name, imports, and build constraints.
// If the file can't be read, an error will be logged, and partial information
// will be returned.
func cueFileInfo(path, rel string) fileInfo {
	info := fileNameInfo(path)
	return info
}
