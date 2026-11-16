package selftest_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/seatbelt/selftest"
	"github.com/hettiger/crannog/internal/testfs"
)

// The tests can write to their temporary directory, also in Seatbelt
// (test.sb), so there the check file can be created as if Seatbelt were
// not active.
func TestRunFailsAndCleansUpWhenTheFileCanBeCreated(t *testing.T) {
	root, dir := testfs.TempRoot(t)

	err := selftest.Run(dir)

	assert.ErrorContains(t, err, "Seatbelt is not active")
	assert.False(t, testfs.Exists(t, root, selftest.CheckFile))
}

// A file that exists says nothing about Seatbelt, and it is not crannog's
// to delete.
func TestRunFailsOnAnExistingFileAndKeepsIt(t *testing.T) {
	root, dir := testfs.TempRoot(t)
	testfs.WriteFile(t, root, selftest.CheckFile, "kept", 0o600)

	err := selftest.Run(dir)

	assert.ErrorContains(t, err, "cannot check that Seatbelt is active")
	assert.Equal(t, "kept", testfs.ReadFile(t, root, selftest.CheckFile))
}

// Only "operation not permitted" passes, not a directory that cannot be
// written to.
func TestRunFailsWhenTheDirectoryIsMissing(t *testing.T) {
	_, dir := testfs.TempRoot(t)

	err := selftest.Run(filepath.Join(dir, "missing"))

	assert.ErrorContains(t, err, "cannot check that Seatbelt is active")
}

func TestTestsOnlyCheckOnMacOS(t *testing.T) {
	_, dir := testfs.TempRoot(t)

	assert.NoError(t, selftest.Tests("linux", dir, nil))
	assert.NoError(t, selftest.Tests("linux", "", errors.New("no home")))
}

func TestTestsRunTheSelfTestOnMacOS(t *testing.T) {
	// Outside of Seatbelt, the home directory can be written to. The
	// temporary directory can be written to even in Seatbelt, so it
	// stands in for it.
	_, writableHome := testfs.TempRoot(t)

	err := selftest.Tests("darwin", writableHome, nil)

	assert.ErrorContains(t, err, "Seatbelt is not active")
	assert.ErrorContains(t, err, "tests must run in Seatbelt (make test)")
}

func TestTestsFailWithoutAHomeOnMacOS(t *testing.T) {
	err := selftest.Tests("darwin", "", errors.New("no home"))

	assert.ErrorContains(t, err, "no home")
}

const selftestPkg = "github.com/hettiger/crannog/internal/seatbelt/selftest"

// Every package with tests imports this package in its selftest_test.go,
// so its test binary runs the self-test when it starts, also when its
// tests access no files and only run a program. The files are those on
// macOS.
func TestEveryPackageWithTestsImportsTheSelfTest(t *testing.T) {
	cmd := exec.Command("go", "list", "-f",
		"{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}} "+
			"{{range .TestGoFiles}} {{.}}{{end}}{{range .XTestGoFiles}} {{.}}{{end}}"+
			"{{range .TestImports}} {{.}}{{end}}{{range .XTestImports}} {{.}}{{end}}{{end}}",
		"github.com/hettiger/crannog/...")
	cmd.Env = append(os.Environ(), "GOOS=darwin")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	require.NoError(t, err)

	var tested, missing []string
	for line := range strings.Lines(string(out)) {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		tested = append(tested, f[0])
		if !slices.Contains(f[1:], "selftest_test.go") || !slices.Contains(f[1:], selftestPkg) {
			missing = append(missing, f[0])
		}
	}
	assert.Contains(t, tested, "github.com/hettiger/crannog/internal/slug")
	assert.Empty(t, missing, "add selftest_test.go with import _ %q (ADR 0030)", selftestPkg)
}
