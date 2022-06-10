package cuelang

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	fileInfoCmpOption = cmp.AllowUnexported(
		fileInfo{},
	)
)

func TestCueFileInfo(t *testing.T) {
	for _, tc := range []struct {
		desc, name, source string
		want               fileInfo
	}{
		{
			"empty file",
			"foo.cue",
			"package foo\n",
			fileInfo{
				packageName: "foo",
			},
		},
		{
			"single import",
			"foo.cue",
			`package foo

import "github.com/foo/bar"
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar"},
			},
		},
		{
			"multiple imports",
			"foo.cue",
			`package foo

import (
	"github.com/foo/bar"
	x "github.com/local/project/y"
)
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar", "github.com/local/project/y"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir, err := ioutil.TempDir(os.Getenv("TEST_TEMPDIR"), "TestCueFileInfo")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, tc.name)
			if err := ioutil.WriteFile(path, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}

			got := cueFileInfo(path, "")
			got = fileInfo{
				packageName: got.packageName,
				imports:     got.imports,
			}
			if diff := cmp.Diff(tc.want, got, fileInfoCmpOption); diff != "" {
				t.Errorf("(-want, +got): %s", diff)
			}
		})
	}
}
