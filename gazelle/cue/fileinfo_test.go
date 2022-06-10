package cuelang

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileNameInfo(t *testing.T) {
	for _, tc := range []struct {
		desc, name string
		want       fileInfo
	}{
		{
			"simple cue file",
			"simple.cue",
			fileInfo{
				ext: cueExt,
			},
		},
		{
			"ignored file",
			"foo.txt",
			fileInfo{
				ext: unknownExt,
			},
		},
		{
			"hidden file",
			".foo.cue",
			fileInfo{
				ext: unknownExt,
			},
		},
	} {
		tc.want.name = tc.name
		tc.want.path = filepath.Join("dir", tc.name)

		if got := fileNameInfo(tc.want.path); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}
