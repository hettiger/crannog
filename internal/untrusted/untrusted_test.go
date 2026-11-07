package untrusted

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("hello"), 0o600))
	require.NoError(t, os.Symlink("a", filepath.Join(dir, "inside")))
	secret := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(secret, filepath.Join(dir, "absolute")))
	require.NoError(t, os.Symlink("../"+filepath.Base(filepath.Dir(secret))+"/secret", filepath.Join(dir, "relative")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dir"), 0o755))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()

	data, info, err := ReadFile(root, "a", 5)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	data, _, err = ReadFile(root, "inside", 5)
	require.NoError(t, err, "a symlink within the root")
	assert.Equal(t, "hello", string(data))

	_, _, err = ReadFile(root, "a", 4)
	assert.ErrorContains(t, err, "larger than 4 bytes")
	for _, name := range []string{"absolute", "relative"} {
		_, _, err = ReadFile(root, name, 100)
		assert.Error(t, err, name)
	}
	for _, name := range []string{"fifo", "dir"} {
		_, _, err = ReadFile(root, name, 100)
		assert.ErrorContains(t, err, "not a regular file", name)
	}
}
