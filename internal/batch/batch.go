// Package batch sets the Matroska segment title on MKV files.
package batch

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gravity-zero/mkvgo/matroska"

	"syncmkv/internal/name"
)

const (
	ActChange = "change"
	ActSame   = "same"
	ActSkip   = "skip"
	ActError  = "error"
)

// Result is one file.
type Result struct {
	Path   string
	Old    string
	New    string
	Kind   string
	Action string
	Reason string
}

// fileOpts controls a write. Rewrite allows a full file copy when the new
// title does not fit in the header. It is off unless the user passes -rewrite.
// series selects "Series - S01E01 - Title". It is off unless the user passes -series.
type fileOpts struct {
	apply   bool
	rewrite bool
	series  bool
}

// File reads one MKV file and sets the segment title when apply is true.
func File(ctx context.Context, path string, opt fileOpts) (Result, error) {
	c, err := matroska.OpenMeta(ctx, path, matroska.WithTags(), matroska.WithChapters(), matroska.WithoutAttachmentData())
	if err != nil {
		return Result{Path: path, Action: ActError}, err
	}
	fields := name.Merge(name.FromGroups(groupsOf(c.Tags)), name.FromPath(path))
	title, kind, reason, ok := name.Desired(fields, opt.series)
	res := Result{Path: path, Old: c.Info.Title, New: title, Kind: kind}
	if !ok {
		res.Action = ActSkip
		res.Reason = reason
		return res, nil
	}
	if c.Info.Title == title {
		res.Action = ActSame
		return res, nil
	}
	res.Action = ActChange
	if !opt.apply {
		return res, nil
	}
	err = matroska.EditInPlace(ctx, path, editor(c, title))
	if needsFullRewrite(err) {
		if !opt.rewrite {
			res.Action = ActSkip
			res.Reason = "needs a full rewrite; pass -rewrite"
			return res, nil
		}
		err = rewrite(ctx, path, c, title)
		res.Reason = "full rewrite"
	}
	if err != nil {
		res.Action = ActError
		return res, err
	}
	return res, nil
}

func groupsOf(tags []matroska.Tag) []name.Group {
	out := make([]name.Group, 0, len(tags))
	for _, t := range tags {
		out = append(out, name.Group{Items: itemsOf(t.SimpleTags)})
	}
	return out
}

func itemsOf(tags []matroska.SimpleTag) []name.Item {
	var out []name.Item
	for _, t := range tags {
		if t.Name != "" && t.Value != "" {
			out = append(out, name.Item{Name: t.Name, Value: t.Value})
		}
		out = append(out, itemsOf(t.SubTags)...)
	}
	return out
}

func needsFullRewrite(err error) bool {
	return err != nil && strings.Contains(err.Error(), "exceeds available space")
}

// editor sets the title and re-applies tags and chapters. The edit calls
// do not read tags, so without this they are dropped on write.
func editor(src *matroska.Container, title string) func(*matroska.Container) {
	return func(edit *matroska.Container) {
		edit.Info.Title = title
		if len(edit.Tags) == 0 {
			edit.Tags = src.Tags
		}
		if len(edit.Chapters) == 0 {
			edit.Chapters = src.Chapters
		}
	}
}

func rewrite(ctx context.Context, path string, src *matroska.Container, title string) error {
	tmp := path + ".syncmkv.tmp"
	err := matroska.EditMetadata(ctx, path, tmp, editor(src, title))
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if st, err := os.Stat(path); err == nil {
		_ = os.Chmod(tmp, st.Mode().Perm())
	}
	if err := replaceFile(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func replaceFile(tmp, dst string) error {
	if runtime.GOOS == "windows" {
		bak := dst + ".syncmkv.bak"
		_ = os.Remove(bak)
		if err := os.Rename(dst, bak); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Rename(bak, dst)
			return err
		}
		return os.Remove(bak)
	}
	return os.Rename(tmp, dst)
}

// Run is the command entry point.
func Run(args []string, stdout, stderr io.Writer) int {
	apply := false
	rewrite := false
	series := false
	recurse := true
	verbose := false
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-apply":
			apply = true
		case a == "-rewrite" || a == "--rewrite":
			rewrite = true
		case a == "-series" || a == "--series":
			series = true
		case a == "-recurse" || a == "--recurse" || a == "-recurse=true" || a == "--recurse=true":
			recurse = true
		case a == "-recurse=false" || a == "--recurse=false":
			recurse = false
		case a == "-v":
			verbose = true
		case a == "-h" || a == "-help" || a == "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "unknown option %s\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, errs := collect(paths, recurse)
	code := 0
	for _, err := range errs {
		fmt.Fprintf(stderr, "ERROR %v\n", err)
		code = 1
	}
	ctx := context.Background()
	var change, same, skip, failed int
	for _, path := range files {
		res, err := File(ctx, path, fileOpts{apply: apply, rewrite: rewrite, series: series})
		switch res.Action {
		case ActChange:
			change++
			word := "WOULD"
			if apply {
				word = "OK"
			}
			fmt.Fprintf(stdout, "%s %s\n  old: %q\n  new: %q\n", word, res.Path, res.Old, res.New)
			if res.Reason != "" {
				fmt.Fprintf(stdout, "  %s\n", res.Reason)
			}
		case ActSame:
			same++
			if verbose {
				fmt.Fprintf(stdout, "SAME %s\n  %q\n", res.Path, res.Old)
			}
		case ActSkip:
			skip++
			fmt.Fprintf(stdout, "SKIP %s\n  %s\n", res.Path, res.Reason)
		default:
			failed++
			code = 1
			fmt.Fprintf(stderr, "ERROR %s\n  %v\n", path, err)
		}
	}
	if !apply {
		fmt.Fprintf(stdout, "dry run, no file was written\n")
	}
	fmt.Fprintf(stdout, "change %d, same %d, skip %d, error %d\n", change, same, skip, failed)
	return code
}

func collect(paths []string, recurse bool) ([]string, []error) {
	var files []string
	var errs []error
	for _, p := range paths {
		if isPattern(p) {
			if err := collectPattern(p, recurse, &files); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !info.IsDir() {
			if !isMKV(p) {
				errs = append(errs, fmt.Errorf("%s: not an MKV file", p))
				continue
			}
			files = append(files, p)
			continue
		}
		if !recurse {
			ents, err := os.ReadDir(p)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, ent := range ents {
				if ent.IsDir() || !isMKV(ent.Name()) {
					continue
				}
				files = append(files, filepath.Join(p, ent.Name()))
			}
			continue
		}
		// The start directory often holds only genre folders. Walk those folders.
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			if isDirectory(d) || !isMKV(path) {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	sort.Strings(files)
	return files, errs
}

func isPattern(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// collectPattern finds MKV files for a pattern such as *.mkv.
// zsh expands an unquoted pattern before this program starts.
func collectPattern(p string, recurse bool, files *[]string) error {
	dir, pat := filepath.Split(p)
	if dir == "" {
		dir = "."
	}
	if _, err := filepath.Match(pat, ""); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	add := func(path, name string) {
		if isMKV(name) && matchName(pat, name) {
			*files = append(*files, path)
		}
	}
	if !recurse {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, ent := range ents {
			if ent.IsDir() {
				continue
			}
			add(filepath.Join(dir, ent.Name()), ent.Name())
		}
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || isDirectory(d) {
			return nil
		}
		add(path, d.Name())
		return nil
	})
}

func matchName(pattern, name string) bool {
	ok, err := filepath.Match(pattern, name)
	if err == nil && ok {
		return true
	}
	ok, err = filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
	return err == nil && ok
}

func isMKV(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".mkv")
}

// isDirectory reports a directory. Some mounts omit the type in the directory
// listing. Stat the entry when the type is unknown.
func isDirectory(d fs.DirEntry) bool {
	if d.Type() != ^fs.FileMode(0) {
		return d.IsDir()
	}
	info, err := d.Info()
	return err == nil && info.IsDir()
}

const usage = `syncmkv sets the segment title of MKV files.

A movie uses the movie title tag.
A series uses the series name, the season number, and the episode number.
The episode name is added when the tags contain it.
A missing tag is read from the folder name and the file name.

The program does not write a file until you pass -apply.

Usage:
  syncmkv [options] [path...]

A directory includes MKV files in its subdirectories.
The top directory does not need to contain an MKV file.
No path means the current directory.
In zsh, quote a file pattern. An unquoted *.mkv stops in zsh when this directory has no MKV file.
  syncmkv -recurse '*.mkv'
  syncmkv .

Options:
  -apply          write the new title
  -series         set a TV title to "Series - S01E01 - Title"
  -rewrite        allow a full file copy when the title does not fit in the header
  -recurse=false  do not enter subdirectories
  -v              show files that already have the correct title
`
