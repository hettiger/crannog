package selftest_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hettiger/crannog/internal/seatbelt/selftest"
	"github.com/hettiger/crannog/internal/testfs"
)

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

func TestInTestsOnlyChecksOnMacOS(t *testing.T) {
	_, dir := testfs.TempRoot(t)

	assert.NoError(t, selftest.Tests("linux", dir, nil))
	assert.NoError(t, selftest.Tests("linux", "", errors.New("no home")))
}

// On macOS a test binary outside of Seatbelt fails, and leaves nothing
// behind.
func TestInTestsFailsOutsideOfSeatbeltOnMacOS(t *testing.T) {
	root, dir := testfs.TempRoot(t)

	err := selftest.Tests("darwin", dir, nil)

	assert.ErrorContains(t, err, "tests must run in Seatbelt (make test)")
	assert.False(t, testfs.Exists(t, root, selftest.CheckFile))
	assert.ErrorContains(t, selftest.Tests("darwin", "", errors.New("no home")), "no home")
}
