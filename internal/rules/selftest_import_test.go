package rules

import (
	"errors"
	"go/build"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

const (
	module      = "github.com/hettiger/crannog"
	selftestPkg = module + "/internal/seatbelt/selftest"
)

// Every package with tests imports package selftest in its
// selftest_test.go, so its test binary runs the self-test of Seatbelt
// when it starts, also when its tests access no files and only run a
// program (ADR 0030). The files are those on macOS.
//
// The test reads the packages itself instead of running go list: the
// test cache only notices changes to files the test process opens, so a
// result of go list would be replayed after a selftest_test.go was
// deleted.
func TestEveryPackageWithTestsImportsTheSelfTest(t *testing.T) {
	repo := testfs.Repo(t)
	ctx := repoContext(repo)
	ctx.GOOS = "darwin"

	var tested, missing []string
	err := fs.WalkDir(repo, ".", func(dir string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if dir != "." && skipDir(repo, dir) {
			return fs.SkipDir
		}
		pkg, err := ctx.ImportDir(dir, 0)
		if _, ok := errors.AsType[*build.NoGoError](err); ok {
			return nil
		}
		if err != nil {
			return err
		}
		files := slices.Concat(pkg.TestGoFiles, pkg.XTestGoFiles)
		if len(files) == 0 {
			return nil
		}
		importPath := path.Join(module, dir)
		tested = append(tested, importPath)
		imports := slices.Concat(pkg.TestImports, pkg.XTestImports)
		if !slices.Contains(files, "selftest_test.go") || !slices.Contains(imports, selftestPkg) {
			missing = append(missing, importPath)
		}
		return nil
	})
	require.NoError(t, err)

	assert.Contains(t, tested, module+"/internal/slug")
	assert.Empty(t, missing, "add selftest_test.go with import _ %q (ADR 0030)", selftestPkg)
}

// repoContext returns a build context that reads the packages from repo,
// by paths relative to it (ADR 0013).
func repoContext(repo fs.FS) build.Context {
	ctx := build.Default
	ctx.JoinPath = path.Join
	ctx.IsAbsPath = path.IsAbs
	ctx.HasSubdir = func(string, string) (string, bool) { return "", false }
	ctx.IsDir = func(p string) bool {
		info, err := fs.Stat(repo, p)
		return err == nil && info.IsDir()
	}
	ctx.ReadDir = func(p string) ([]fs.FileInfo, error) {
		entries, err := fs.ReadDir(repo, p)
		if err != nil {
			return nil, err
		}
		infos := make([]fs.FileInfo, len(entries))
		for i, e := range entries {
			if infos[i], err = e.Info(); err != nil {
				return nil, err
			}
		}
		return infos, nil
	}
	ctx.OpenFile = func(p string) (io.ReadCloser, error) { return repo.Open(p) }
	return ctx
}

// skipDir reports whether the pattern ./... leaves a directory out: names
// starting with "." or "_", testdata, vendor and nested modules.
func skipDir(repo fs.FS, dir string) bool {
	name := path.Base(dir)
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "vendor" {
		return true
	}
	_, err := fs.Stat(repo, path.Join(dir, "go.mod"))
	return err == nil
}
