// Package setup reads, hashes and stores the content of a .crannog/
// directory (ADR 0023, ADR 0025, ADR 0026).
package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/hostfs"
)

// Dir is the name of the setup directory in a repository.
const Dir = hostfs.SetupDir

// Paths of the files crannog reads from a setup.
const (
	ConfigFile     = "config.yml"
	ProxyFile      = "iron-proxy.yaml"
	ContainerDir   = "container"
	DockerfileFile = "container/Dockerfile"
)

// File is one file of a setup.
type File struct {
	Data       []byte
	Executable bool
}

// Setup is the content of a .crannog/ directory: files by slash-separated
// path relative to .crannog/.
type Setup map[string]File

// Hash returns the setup hash: sha256 over the sorted paths, the
// executable bits and the contents, in hex.
func (s Setup) Hash() string {
	h := sha256.New()
	for _, p := range s.paths() {
		f := s[p]
		x := "-"
		if f.Executable {
			x = "x"
		}
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", p, x, len(f.Data))
		h.Write(f.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Container returns the files of container/, relative to it.
func (s Setup) Container() Setup {
	c := Setup{}
	for p, f := range s {
		if rest, ok := strings.CutPrefix(p, ContainerDir+"/"); ok {
			c[rest] = f
		}
	}
	return c
}

// ImageName returns the name of the image built from container/:
// crannog-<first twelve characters of the hash of container/ alone>.
func (s Setup) ImageName() string {
	return "crannog-" + s.Container().Hash()[:12]
}

// Without returns a copy of the setup without the given paths.
func (s Setup) Without(paths ...string) Setup {
	c := Setup{}
	for p, f := range s {
		c[p] = f
	}
	for _, p := range paths {
		delete(c, p)
	}
	return c
}

// Equal reports whether two setups have the same content.
func (s Setup) Equal(o Setup) bool {
	if len(s) != len(o) {
		return false
	}
	for p, f := range s {
		g, ok := o[p]
		if !ok || f.Executable != g.Executable || !bytes.Equal(f.Data, g.Data) {
			return false
		}
	}
	return true
}

// Validate checks that the setup has the files crannog needs and that its
// text files show in the approval diff as crannog reads them.
func (s Setup) Validate() error {
	var missing []string
	for _, p := range []string{ConfigFile, ProxyFile, DockerfileFile} {
		if _, ok := s[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s/ lacks %s", Dir, strings.Join(missing, ", "))
	}
	for _, p := range s.paths() {
		if isBinary(s[p].Data) {
			continue
		}
		if err := checkControl(s[p].Data); err != nil {
			return fmt.Errorf("%s/%s: %w", Dir, p, err)
		}
	}
	return nil
}

// isBinary reports whether git diff shows data as binary rather than as
// text: a NUL byte within the first 8000 bytes.
func isBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}

// checkControl refuses control characters other than tab and line breaks
// in a text file: escape sequences could hide lines of the approval diff
// in the terminal.
func checkControl(data []byte) error {
	for i, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' || b == 0x7f {
			line := bytes.Count(data[:i], []byte("\n")) + 1
			return fmt.Errorf("line %d: control character %#02x", line, b)
		}
	}
	return nil
}

func (s Setup) paths() []string {
	ps := make([]string, 0, len(s))
	for p := range s {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps
}

// Write writes the setup into the directory dir within root, which must not
// exist yet.
func (s Setup) Write(root *os.Root, dir string) error {
	if err := root.Mkdir(dir, 0o755); err != nil {
		return err
	}
	for _, p := range s.paths() {
		f := s[p]
		if !validPath(p) {
			return fmt.Errorf("invalid path in setup: %q", p)
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := root.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f.Executable {
			mode = 0o755
		}
		if err := root.WriteFile(target, f.Data, mode); err != nil {
			return err
		}
		if err := root.Chmod(target, mode); err != nil {
			return err
		}
	}
	return nil
}

// ReadDir reads a setup from the directory dir within root, e.g. an
// approved copy or a user preset. Symlinks are an
// error.
func ReadDir(root *os.Root, dir string) (Setup, error) {
	r, err := hostfs.OpenDir(root, dir, false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	s := Setup{}
	err = fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case p == ".":
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s/%s is a symlink", dir, p)
		case d.IsDir():
			return nil
		case !d.Type().IsRegular():
			return fmt.Errorf("%s/%s is not a regular file", dir, p)
		}
		f, err := readFile(r, p)
		if err != nil {
			return err
		}
		s[p] = f
		return nil
	})
	return s, err
}

// ReadFS reads a setup from a file system, e.g. a bundled preset. Files of
// an embedded file system are never executable.
func ReadFS(fsys fs.FS) (Setup, error) {
	s := Setup{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		s[p] = File{Data: data, Executable: info.Mode()&0o100 != 0}
		return nil
	})
	return s, err
}

// readFile reads the file name within root.
func readFile(root *os.Root, name string) (File, error) {
	f, err := root.Open(name)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return File{}, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return File{}, err
	}
	return File{Data: data, Executable: info.Mode()&0o100 != 0}, nil
}

// openSetup opens .crannog/ in the worktree at dir as a root (ADR 0013).
func openSetup(dir string) (*os.Root, error) {
	wt, err := hostfs.OpenWorktree(dir)
	if err != nil {
		return nil, err
	}
	defer wt.Close()
	return wt.OpenSetup()
}

// Present reports whether the worktree at dir has a .crannog/ (or a file
// of that name).
func Present(dir string) (bool, error) {
	wt, err := hostfs.OpenWorktree(dir)
	if err != nil {
		return false, err
	}
	defer wt.Close()
	_, err = wt.LstatSetup()
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// checkDirectory fails if the worktree at dir has a .crannog that exists
// and is not a directory, symlinks included. A missing one is fine.
func checkDirectory(dir string) error {
	wt, err := hostfs.OpenWorktree(dir)
	if err != nil {
		return err
	}
	defer wt.Close()
	info, err := wt.LstatSetup()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", Dir)
	}
	return nil
}

// ReadWorktree reads .crannog/ from the working tree of the worktree at
// dir, uncommitted changes included. Only files git lists as tracked or
// untracked and not ignored count; files deleted in the working tree do
// not. Symlinks are an error, and so is a .crannog that is a file or a
// symlink. A missing .crannog/ gives an empty setup.
func ReadWorktree(dir string) (Setup, error) {
	out, err := git.Output(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", Dir)
	if err != nil {
		return nil, err
	}
	s := Setup{}
	var root *os.Root
	seen := map[string]bool{}
	dirs := map[string]bool{}
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if name == Dir {
			// A file or symlink named .crannog, not a directory.
			if err := checkDirectory(dir); err != nil {
				return nil, err
			}
			continue
		}
		rel, ok := strings.CutPrefix(name, Dir+"/")
		if !ok {
			continue
		}
		if root == nil {
			root, err = openSetup(dir)
			if errors.Is(err, fs.ErrNotExist) {
				// .crannog/ was deleted in the working tree.
				return s, nil
			}
			if err != nil {
				return nil, err
			}
			defer root.Close()
		}
		if err := checkDirs(root, rel, dirs); err != nil {
			return nil, err
		}
		info, err := root.Lstat(rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is a symlink", name)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory, e.g. a nested repository", name)
		}
		f, err := readFile(root, rel)
		if err != nil {
			return nil, err
		}
		s[rel] = f
	}
	return s, nil
}

// checkDirs fails if a directory on the way to the file rel within root is
// a symlink: an os.Root follows symlinks that stay within it. checked
// remembers the directories already checked.
func checkDirs(root *os.Root, rel string, checked map[string]bool) error {
	for dir := path.Dir(rel); dir != "."; dir = path.Dir(dir) {
		if checked[dir] {
			return nil
		}
		info, err := root.Lstat(dir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err == nil && info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s/%s is a symlink", Dir, dir)
		}
		checked[dir] = true
	}
	return nil
}

// ReadCommit reads .crannog/ from a commit of the repository at dir.
// Symlinks and submodules are an error, and so is a .crannog that is not a
// directory. A missing .crannog/ gives an empty
// setup.
func ReadCommit(dir, rev string) (Setup, error) {
	out, err := git.Output(dir, "ls-tree", "-r", "-z", "--full-tree", rev, "--", Dir)
	if err != nil {
		return nil, err
	}
	s := Setup{}
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		meta, name, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("unexpected git ls-tree output: %q", entry)
		}
		if name == Dir {
			return nil, fmt.Errorf("%s in %s is not a directory (mode %s)", name, rev, fields[0])
		}
		rel, ok := strings.CutPrefix(name, Dir+"/")
		if !ok {
			continue
		}
		mode, kind, object := fields[0], fields[1], fields[2]
		if kind != "blob" || (mode != "100644" && mode != "100755") {
			return nil, fmt.Errorf("%s in %s is not a regular file (mode %s)", name, rev, mode)
		}
		data, err := git.Output(dir, "cat-file", "blob", object)
		if err != nil {
			return nil, err
		}
		s[rel] = File{Data: data, Executable: mode == "100755"}
	}
	return s, nil
}

// HasChanges reports whether the worktree at dir has uncommitted changes
// to .crannog/, untracked files included.
func HasChanges(dir string) (bool, error) {
	out, err := git.Run(dir, "status", "--porcelain", "--untracked-files=all", "--", Dir)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// validPath reports whether p is a clean, relative, slash-separated path.
func validPath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && path.Clean(p) == p && !strings.HasPrefix(p, "../") && p != ".."
}
