package batch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"-rewrite", t.TempDir()}, &out, &errb); code != 0 {
		t.Fatalf("code %d %s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := Run([]string{"-no-such"}, &out, &errb); code != 2 {
		t.Fatalf("code %d", code)
	}
}

func TestCollectQuotedGlob(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "Drama")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	mkv := filepath.Join(nested, "Rear Window (1954).mkv")
	if err := os.WriteFile(mkv, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	files, errs := collect([]string{"*.mkv"}, true)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	want := filepath.Join("Drama", "Rear Window (1954).mkv")
	if len(files) != 1 || files[0] != want {
		t.Fatalf("files %v", files)
	}
}

func TestCollectNestedMKV(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "Action", "007")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	mkv := filepath.Join(nested, "Moonraker (1979).mkv")
	if err := os.WriteFile(mkv, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "poster.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, errs := collect([]string{root}, true)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(files) != 1 || files[0] != mkv {
		t.Fatalf("files %v", files)
	}
}

func TestRealFiles(t *testing.T) {
	cases := []struct {
		path string
		new  string
		kind string
		same bool
	}{
		{
			path: "/mnt/zulu/tv/andromeda/Season 01/Andromeda - S01E01 - Under the Night.mkv",
			new:  "Under the Night (1)",
			kind: "series",
		},
		{
			path: "/mnt/zulu/movies/war/Navarone Collection/The Guns Of Navarone (1961).mkv",
			new:  "The Guns of Navarone",
			kind: "movie",
		},
		{
			path: "/mnt/zulu/movies/war/Hart's War (2002).mkv",
			new:  "Hart's War",
			kind: "movie",
			same: true,
		},
		{
			path: "/mnt/zulu/movies/war/The Pilot_ A Battle for Survival (2021).mkv",
			new:  "The Pilot: A Battle for Survival",
			kind: "movie",
		},
	}
	for _, tc := range cases {
		if _, err := os.Stat(tc.path); err != nil {
			t.Skip(tc.path)
		}
		res, err := File(context.Background(), tc.path, fileOpts{})
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if res.New != tc.new || res.Kind != tc.kind {
			t.Fatalf("%s: got %q %s want %q %s", tc.path, res.New, res.Kind, tc.new, tc.kind)
		}
		if tc.same && res.Action != ActSame {
			t.Fatalf("%s: action %s old %q", tc.path, res.Action, res.Old)
		}
	}
}
