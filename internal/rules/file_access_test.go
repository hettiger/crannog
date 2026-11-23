package rules

import (
	"fmt"
	"go/ast"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pathFuncs are the functions that access the file system by path. Code
// must use an os.Root instead (ADR 0013).
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

// allowedFileAccess are the uses of path functions outside of package
// hostfs, by file and function, with the reason.
var allowedFileAccess = map[string]string{
	// The anchors of the tests (ADR 0013).
	"internal/testfs/testfs.go os.OpenRoot":           "opens a temporary directory or the repository",
	"internal/testfs/testfs.go filepath.EvalSymlinks": "resolves a directory before checking it is temporary",
	// The self-test of Seatbelt (ADR 0030): it must fail to create a file
	// outside the anchors, and deletes the file if it could.
	"internal/seatbelt/selftest/selftest.go os.OpenFile": "tries to create the check file in the home directory",
	"internal/seatbelt/selftest/selftest.go os.Remove":   "deletes the check file if it could be created",
	// Read-only checks outside the anchors.
	"internal/crannog/lifecycle.go os.Stat": "checks that approved mount sources exist",
}

// Every file system access by path outside of package hostfs is one of
// the allowed ones. Tests follow the same rule, those of hostfs too.
func TestFileSystemAccessIsAnchored(t *testing.T) {
	var violations []string
	goFiles(t, func(name string, src []byte) {
		if path.Dir(name) == "internal/hostfs" && !strings.HasSuffix(name, "_test.go") {
			return
		}
		violations = append(violations, fileAccess(t, name, src)...)
	})
	assert.Empty(t, violations, "use an os.Root (ADR 0013)")
}

// fileAccess returns the uses of path functions in a file that are not
// allowed.
func fileAccess(t *testing.T, name string, src []byte) []string {
	fset, f, imports := parse(t, name, src)

	var out []string
	report := func(n ast.Node, what string) {
		pkg, _, _ := strings.Cut(what, ".")
		if _, ok := allowedFileAccess[name+" "+what]; ok {
			return
		}
		if _, ok := allowedFileAccess[name+" "+pkg]; ok {
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
		return true
	})
	return out
}
