package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	"internal/runtime/runtime.go syscall.Exec":          "replaces the remote helper and crannog ssh-proxy with container exec",
	"internal/seatbelt/seatbelt_darwin.go syscall.Exec": "replaces the process with sandbox-exec",
	"internal/openssh/openssh.go os/exec":               "runs ssh -G",
	"internal/openssh/openssh.go syscall.Exec":          "replaces crannog ssh with ssh",
	"internal/open/open.go os/exec":                     "runs open with crannog's URLs",
	"internal/probe/probe.go net":                       "checks that a server in a crannog accepts connections",
	// Tests.
	"internal/testgit/testgit.go os/exec":      "runs git below the temporary directory",
	"internal/crannog/scripts_test.go os/exec": "runs with sh the scripts crannog runs in a crannog",
	"internal/shell/shell_test.go os/exec":     "checks the quoting with sh",
	"internal/session/session_test.go os/exec": "runs cat as a child that keeps the session file open",
	"internal/probe/probe_test.go net":         "listens on localhost for the check",
}

// Every process start is in one of the allowed files. Tests follow the
// same rule.
func TestProcessStartsAreAllowed(t *testing.T) {
	var violations []string
	used := map[string]bool{}
	goFiles(t, func(name string, src []byte) {
		for _, v := range processStarts(t, name, src) {
			key := name + " " + v.what
			if _, ok := allowedProcesses[key]; ok {
				used[key] = true
				continue
			}
			violations = append(violations, fmt.Sprintf("%s: %s", v.pos, v.what))
		}
	})
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
	fset, f, imports := parse(t, name, src)

	var out []processStart
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		if slices.Contains(processImports, p) {
			out = append(out, processStart{fset.Position(imp.Pos()), p})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		for pkg, funcs := range processFuncs {
			if selects(imports, sel, pkg, funcs...) {
				out = append(out, processStart{fset.Position(sel.Pos()), pkg + "." + sel.Sel.Name})
			}
		}
		return true
	})
	return out
}
