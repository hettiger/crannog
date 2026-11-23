// Package rules holds rules for the whole repository that no single
// package owns.
package rules

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

// processImports are the packages that start processes or open network
// connections. A file that imports one must be allowed (ADR 0031).
var processImports = []string{"os/exec", "net", "net/http", "golang.org/x/sys/unix"}

// processFuncs are the functions that start processes in packages that
// are imported for other things too.
var processFuncs = map[string][]string{
	"os":      {"StartProcess"},
	"syscall": {"Exec", "ForkExec", "StartProcess"},
}

// allowedProcesses are the files that may start processes, by file and
// import or function, with what they run.
var allowedProcesses = map[string]string{
	// The one place of each program crannog runs on the host.
	"internal/git/git.go os/exec":                       "runs git",
	"internal/runtime/runtime.go os/exec":               "runs container",
	"internal/runtime/runtime.go syscall.Exec":          "replaces the remote helper with container exec",
	"internal/seatbelt/seatbelt_darwin.go syscall.Exec": "replaces the process with sandbox-exec",
	// Tests.
	"internal/testgit/testgit.go os/exec":      "runs git below the temporary directory",
	"internal/crannog/scripts_test.go os/exec": "runs with sh the scripts crannog runs in a crannog",
	"internal/shell/shell_test.go os/exec":     "checks the quoting with sh",
	"internal/session/session_test.go os/exec": "runs cat as a child that keeps the session file open",
}

// Every process start is in one of the allowed files. Tests follow the
// same rule.
func TestProcessStartsAreAllowed(t *testing.T) {
	repo := testfs.Repo(t)
	var violations []string
	used := map[string]bool{}
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
		for _, v := range processStarts(t, p, src) {
			key := p + " " + v.what
			if _, ok := allowedProcesses[key]; ok {
				used[key] = true
				continue
			}
			violations = append(violations, fmt.Sprintf("%s: %s", v.pos, v.what))
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, violations, "start processes in the package of their program (ADR 0031)")
	for key := range allowedProcesses {
		assert.True(t, used[key], "%s is allowed but not used", key)
	}
}

func TestProcessStartsAreFound(t *testing.T) {
	src := `package x

import (
	"os"
	run "os/exec"
	"net/http"
	sys "syscall"
)

var _ = run.Command
var _ = http.Get
var _ = os.StartProcess
var _ = sys.ForkExec
var _ = sys.Getpid
var _ = os.Getenv
`
	var found []string
	for _, v := range processStarts(t, "x.go", []byte(src)) {
		found = append(found, v.what)
	}
	assert.Equal(t, []string{"os/exec", "net/http", "os.StartProcess", "syscall.ForkExec"}, found)
}

type processStart struct {
	pos token.Position
	// what is the import path, or package.Function.
	what string
}

// processStarts returns the imports and function uses in a file that can
// start a process or open a network connection.
func processStarts(t *testing.T, name string, src []byte) []processStart {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	require.NoError(t, err)

	var out []processStart
	imports := map[string]string{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		local := path.Base(p)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		imports[local] = p
		for _, forbidden := range processImports {
			if p == forbidden {
				out = append(out, processStart{fset.Position(imp.Pos()), p})
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil {
			return true
		}
		pkg := imports[id.Name]
		for _, fn := range processFuncs[pkg] {
			if sel.Sel.Name == fn {
				out = append(out, processStart{fset.Position(sel.Pos()), pkg + "." + fn})
			}
		}
		return true
	})
	return out
}
