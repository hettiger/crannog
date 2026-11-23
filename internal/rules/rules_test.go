// Package rules holds the rules for the whole repository that a compiler
// cannot check: where files are accessed (ADR 0013), where processes are
// started (ADR 0031) and that the tests of every package run the self-test
// of Seatbelt (ADR 0030). It consists of tests only.
package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

const renameio = "github.com/google/renameio/v2"

// goFiles calls fn with every Go file of the repository, code and tests
// alike, by its path in the repository.
func goFiles(t *testing.T, fn func(name string, src []byte)) {
	repo := testfs.Repo(t)
	err := fs.WalkDir(repo, ".", func(p string, d fs.DirEntry, err error) error {
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
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		src, err := fs.ReadFile(repo, p)
		if err != nil {
			return err
		}
		fn(p, src)
		return nil
	})
	require.NoError(t, err)
}

// parse parses a Go file and returns its imports by the names the file
// uses them with.
func parse(t *testing.T, name string, src []byte) (*token.FileSet, *ast.File, map[string]string) {
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
	return fset, f, imports
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
