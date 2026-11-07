package untrusted

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

func TestReadFile(t *testing.T) {
	tmp, _ := testfs.TempRoot(t)
	require.NoError(t, tmp.Mkdir("dir", 0o755))
	require.NoError(t, tmp.Mkdir("outside", 0o755))
	testfs.WriteFile(t, tmp, "outside/secret", "secret", 0o600)
	dir, err := tmp.OpenRoot("dir")
	require.NoError(t, err)
	defer dir.Close()

	testfs.WriteFile(t, dir, "a", "hello", 0o600)
	require.NoError(t, dir.Symlink("a", "inside"))
	require.NoError(t, dir.Symlink(filepath.Join(tmp.Name(), "outside", "secret"), "absolute"))
	require.NoError(t, dir.Symlink("../outside/secret", "relative"))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir.Name(), "fifo"), 0o600))
	require.NoError(t, dir.Mkdir("dir", 0o755))

	data, info, err := ReadFile(dir, "a", 5)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	data, _, err = ReadFile(dir, "inside", 5)
	require.NoError(t, err, "a symlink within the root")
	assert.Equal(t, "hello", string(data))

	_, _, err = ReadFile(dir, "a", 4)
	assert.ErrorContains(t, err, "larger than 4 bytes")
	for _, name := range []string{"absolute", "relative"} {
		_, _, err = ReadFile(dir, name, 100)
		assert.Error(t, err, name)
	}
	for _, name := range []string{"fifo", "dir"} {
		_, _, err = ReadFile(dir, name, 100)
		assert.ErrorContains(t, err, "not a regular file", name)
	}
}
