package hostfs

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// pathFuncs are the functions that access the file system by path. Code
// must use an os.Root instead (ADR 0031).
var pathFuncs = map[string][]string{
	"os": {"Chdir", "Chmod", "Chown", "Chtimes", "CopyFS", "Create", "CreateTemp", "DirFS",
		"Lchown", "Link", "Lstat", "Mkdir", "MkdirAll", "MkdirTemp", "Open", "OpenFile",
		"OpenInRoot", "OpenRoot", "ReadDir", "ReadFile", "Readlink", "Remove", "RemoveAll",
		"Rename", "Stat", "Symlink", "Truncate", "WriteFile"},
	"io/ioutil":     {"ReadDir", "ReadFile", "TempDir", "TempFile", "WriteFile"},
	"path/filepath": {"EvalSymlinks", "Glob", "Walk", "WalkDir"},
	"syscall": {"Chdir", "Chmod", "Chown", "Creat", "Lchown", "Link", "Lstat", "Mkdir",
		"Mkdirat", "Mkfifo", "Mknod", "Open", "Openat", "Readlink", "Rename", "Renameat",
		"Rmdir", "Stat", "Symlink", "Truncate", "Unlink", "Unlinkat"},
	"github.com/stretchr/testify/assert":  {"DirExists", "FileExists", "NoDirExists", "NoFileExists"},
	"github.com/stretchr/testify/require": {"DirExists", "FileExists", "NoDirExists", "NoFileExists"},
}

// renameioFuncs write by path unless they get renameio.WithRoot.
var renameioFuncs = []string{"NewPendingFile", "Symlink", "TempFile", "WriteFile"}

const renameio = "github.com/google/renameio/v2"

// allowed are the uses of path functions outside this package, by file
// and function, with the reason.
var allowed = map[string]string{
	// The anchor of the tests (ADR 0031).
	"internal/testfs/testfs.go os.OpenRoot": "opens a test's temporary directory",
	// Read-only checks outside the anchors.
	"internal/crannog/lifecycle.go os.Stat":             "checks that approved mount sources exist",
	"internal/testgit/testgit.go filepath.EvalSymlinks": "resolves the test's temporary directory",
}

// anchorUsers may open anchors with OpenAnchor.
var anchorUsers = map[string]string{
	"internal/home/home.go":          "~/.crannog",
	"internal/setup/setup.go":        ".crannog/ in a worktree",
	"internal/preset/preset.go":      ".crannog/ in a worktree",
	"internal/hostfs/hostfs_test.go": "tests OpenAnchor",
}

// Every file system access by path outside this package is one of the
// allowed ones. Tests follow the same rule.
func TestFileSystemAccessIsAnchored(t *testing.T) {
	root := testfs.Root(t, "../..")
	var violations []string
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".tickets", "dist", "e2e", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || (path.Dir(p) == "internal/hostfs" && !strings.HasSuffix(p, "_test.go")) {
			return nil
		}
		src, err := root.ReadFile(p)
		if err != nil {
			return err
		}
		violations = append(violations, check(t, p, src)...)
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, violations, "use an os.Root (ADR 0031)")
}

func check(t *testing.T, name string, src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	require.NoError(t, err)
	imports := map[string]string{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		local := path.Base(p)
		if p == renameio {
			local = "renameio"
		}
		if imp.Name != nil {
			local = imp.Name.Name
		}
		imports[local] = p
	}

	var out []string
	report := func(n ast.Node, what string) {
		pkg, _, _ := strings.Cut(what, ".")
		if _, ok := allowed[name+" "+what]; ok {
			return
		}
		if _, ok := allowed[name+" "+pkg]; ok {
			return
		}
		out = append(out, fmt.Sprintf("%s: %s", fset.Position(n.Pos()), what))
	}
	withRoot := map[*ast.SelectorExpr]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && selects(imports, sel, renameio, renameioFuncs...) {
				for _, arg := range call.Args {
					if c, ok := arg.(*ast.CallExpr); ok {
						if s, ok := c.Fun.(*ast.SelectorExpr); ok && selects(imports, s, renameio, "WithRoot") {
							withRoot[sel] = true
						}
					}
				}
			}
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		what := id.Name + "." + sel.Sel.Name
		for pkg, funcs := range pathFuncs {
			if selects(imports, sel, pkg, funcs...) {
				report(sel, what)
			}
		}
		if selects(imports, sel, renameio, renameioFuncs...) && !withRoot[sel] {
			report(sel, what+" without WithRoot")
		}
		if selects(imports, sel, "github.com/hettiger/crannog/internal/hostfs", "OpenAnchor") {
			if _, ok := anchorUsers[name]; !ok {
				report(sel, what)
			}
		}
		return true
	})
	return out
}

// selects reports whether sel selects one of funcs of the package with
// import path pkg.
func selects(imports map[string]string, sel *ast.SelectorExpr, pkg string, funcs ...string) bool {
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Obj != nil || imports[id.Name] != pkg {
		return false
	}
	for _, f := range funcs {
		if sel.Sel.Name == f {
			return true
		}
	}
	return false
}
