package preset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/setup"
)

func TestBundledDefault(t *testing.T) {
	p, err := Find(t.TempDir(), Default)
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
	dir := t.TempDir()
	p, err := Find(t.TempDir(), Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".crannog", "old"), 0o755))
	require.NoError(t, Publish(s, dir))
	got, err := setup.ReadDir(filepath.Join(dir, ".crannog"))
	require.NoError(t, err)
	assert.Equal(t, s.Hash(), got.Hash(), "replaced completely")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary dir left")
}

func TestUserPresets(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		for _, f := range []string{"config.yml", "iron-proxy.yaml", "container/Dockerfile"} {
			p := filepath.Join(dir, name, f)
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
			require.NoError(t, os.WriteFile(p, []byte(name+" "+f), 0o644))
		}
	}
	write("default")
	write("laravel")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(dir, "laravel"), filepath.Join(dir, "link")))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "incomplete"), 0o755))

	list, err := List(dir)
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

	p, err := Find(dir, Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)
	assert.Equal(t, "default config.yml", string(s["config.yml"].Data))
	assert.NotContains(t, s, "container/entrypoint.sh", "replaces the bundled preset completely")

	p, err = Find(dir, "incomplete")
	require.NoError(t, err)
	_, err = p.Load()
	assert.ErrorContains(t, err, "lacks")

	_, err = Find(dir, "missing")
	assert.ErrorContains(t, err, "no preset named")
}
