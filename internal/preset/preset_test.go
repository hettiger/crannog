package preset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/testfs"
)

func tempHome(t *testing.T) home.Home {
	h, err := home.Open(filepath.Join(t.TempDir(), ".crannog"))
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

func TestBundledDefault(t *testing.T) {
	p, err := Find(tempHome(t), Default)
	require.NoError(t, err)
	assert.False(t, p.User)
	assert.Equal(t, "bundled", p.Source())

	s, err := p.Load()
	require.NoError(t, err)
	assert.Contains(t, s, "container/entrypoint.sh")

	c, err := config.Parse(s[setup.ConfigFile].Data)
	require.NoError(t, err)
	assert.Empty(t, c.HostVariables(), "the default preset reads nothing from the host")
	_, err = c.Expand(config.Vars{Crannog: map[string]string{
		config.AgentsDir: "/a", config.CacheDir: "/c", config.CADir: "/ca",
		config.RepoDir: "/work/r", config.ProxyURL: config.ProxyURLValue,
	}}, "/Users/me")
	require.NoError(t, err)
}

func TestPublish(t *testing.T) {
	wt, dir := testfs.TempRoot(t)
	p, err := Find(tempHome(t), Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)

	require.NoError(t, wt.MkdirAll(".crannog/old", 0o755))
	require.NoError(t, Publish(s, dir))
	got, err := setup.ReadDir(wt, ".crannog")
	require.NoError(t, err)
	assert.Equal(t, s.Hash(), got.Hash(), "replaced completely")

	entries, err := fs.ReadDir(wt.FS(), ".")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary dir left")
}

// A symlinked .crannog is replaced; its target stays untouched.
func TestPublishReplacesASymlink(t *testing.T) {
	wt, dir := testfs.TempRoot(t)
	testfs.WriteFile(t, wt, "elsewhere/keep", "keep", 0o644)
	require.NoError(t, wt.Symlink("elsewhere", ".crannog"))
	p, err := Find(tempHome(t), Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)

	require.NoError(t, Publish(s, dir))
	info, err := wt.Lstat(".crannog")
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, "keep", testfs.ReadFile(t, wt, "elsewhere/keep"))
}

func publishFixture(t *testing.T) (*os.Root, string, setup.Setup) {
	wt, dir := testfs.TempRoot(t)
	p, err := Find(tempHome(t), Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)
	testfs.WriteFile(t, wt, ".crannog/old.txt", "old", 0o644)
	return wt, dir, s
}

func TestPublishWithoutExistingSetup(t *testing.T) {
	wt, dir := testfs.TempRoot(t)
	p, err := Find(tempHome(t), Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)

	require.NoError(t, Publish(s, dir))
	got, err := setup.ReadDir(wt, ".crannog")
	require.NoError(t, err)
	assert.Equal(t, s.Hash(), got.Hash())
	entries, err := fs.ReadDir(wt.FS(), ".")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary dir left")
}

// The old .crannog stays until the new one is ready, and comes back if
// installing the new one fails.
func TestPublishRestoresTheOldSetup(t *testing.T) {
	wt, dir, s := publishFixture(t)
	orig := rename
	t.Cleanup(func() { rename = orig })
	failed := false
	rename = func(root *os.Root, oldname, newname string) error {
		// Only the first move into place fails, the restore works.
		if newname == ".crannog" && strings.HasSuffix(oldname, "/.crannog") && !failed {
			failed = true
			return errors.New("injected rename failure")
		}
		return orig(root, oldname, newname)
	}

	err := Publish(s, dir)
	require.ErrorContains(t, err, "injected rename failure")
	assert.Equal(t, "old", testfs.ReadFile(t, wt, ".crannog/old.txt"))
	entries, err := fs.ReadDir(wt.FS(), ".")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary dir left")
}

// If the old setup cannot be restored either, the backup is kept and the
// error says where.
func TestPublishKeepsTheBackupIfRestoringFails(t *testing.T) {
	wt, dir, s := publishFixture(t)
	orig := rename
	t.Cleanup(func() { rename = orig })
	rename = func(root *os.Root, oldname, newname string) error {
		// Moving the new setup into place and restoring the backup fail.
		if newname == ".crannog" {
			return errors.New("injected rename failure")
		}
		return orig(root, oldname, newname)
	}

	err := Publish(s, dir)
	require.ErrorContains(t, err, "restoring the old .crannog failed")
	assert.False(t, testfs.Exists(t, wt, ".crannog"))

	entries, rerr := fs.ReadDir(wt.FS(), ".")
	require.NoError(t, rerr)
	require.Len(t, entries, 1)
	tmp := entries[0].Name()
	assert.Contains(t, err.Error(), filepath.Join(wt.Name(), tmp, "old"))
	assert.Equal(t, "old", testfs.ReadFile(t, wt, tmp+"/old/old.txt"))
}

func TestUserPresets(t *testing.T) {
	h := tempHome(t)
	require.NoError(t, h.Root.Mkdir(home.Presets, 0o755))
	dir := testfs.Root(t, h.Abs(home.Presets))
	write := func(name string) {
		for _, f := range []string{"config.yml", "iron-proxy.yaml", "container/Dockerfile"} {
			testfs.WriteFile(t, dir, name+"/"+f, name+" "+f, 0o644)
		}
	}
	write("default")
	write("laravel")
	require.NoError(t, dir.Mkdir(".hidden", 0o755))
	require.NoError(t, dir.Symlink("laravel", "link"))
	require.NoError(t, dir.Mkdir("incomplete", 0o755))

	list, err := List(h)
	require.NoError(t, err)
	var names []string
	for _, p := range list {
		names = append(names, p.Name+" ("+p.Source()+")")
	}
	assert.Equal(t, []string{
		"default (user, replaces bundled)",
		"incomplete (user)",
		"laravel (user)",
	}, names)

	p, err := Find(h, Default)
	require.NoError(t, err)
	assert.Equal(t, h.Abs("presets/default"), p.Dir)
	s, err := p.Load()
	require.NoError(t, err)
	assert.Equal(t, "default config.yml", string(s["config.yml"].Data))
	assert.NotContains(t, s, "container/entrypoint.sh", "replaces the bundled preset completely")

	p, err = Find(h, "incomplete")
	require.NoError(t, err)
	_, err = p.Load()
	assert.ErrorContains(t, err, "lacks")

	_, err = Find(h, "missing")
	assert.ErrorContains(t, err, "no preset named")
}
