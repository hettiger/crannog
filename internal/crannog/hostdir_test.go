package crannog

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
)

// unrelated creates a directory with files that look like a crannog's
// state, in the user's home outside ~/.crannog. Nothing may change it.
func (f *fixture) unrelated() (user *os.Root, check func()) {
	user = f.userRoot()
	for _, name := range []string{"Documents/unrelated.txt", "branch", "sessions/12345", "sessions/unrelated.txt"} {
		testfs.WriteFile(f.t, user, filepath.Join("unrelated", name), "keep", 0o644)
	}
	return user, func() {
		f.t.Helper()
		var names []string
		require.NoError(f.t, fs.WalkDir(user.FS(), "unrelated", func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				names = append(names, p)
			}
			return err
		}))
		assert.ElementsMatch(f.t, []string{
			"unrelated/Documents/unrelated.txt", "unrelated/branch",
			"unrelated/sessions/12345", "unrelated/sessions/unrelated.txt",
		}, names, "the unrelated directory is untouched")
	}
}

// A symlink in crannog's state never redirects a cleanup, a lock or a write
// (ADR 0032): the crannog directory itself, a directory on the way to it
// or one in it.
func TestHostDirRefusesSymlinks(t *testing.T) {
	// The symlink and its target, relative to the symlink.
	links := map[string]func(f *fixture, tg *target) (string, string){
		"crannog directory": func(f *fixture, tg *target) (string, string) {
			return tg.dir.path, "../../../../unrelated"
		},
		"repository directory": func(f *fixture, tg *target) (string, string) {
			return home.RepositoryPath(f.m.Repo.ID), "../../unrelated"
		},
		"sessions directory": func(f *fixture, tg *target) (string, string) {
			return filepath.Join(tg.dir.path, sessionsDir), "../../../../../unrelated/sessions"
		},
	}
	for name, link := range links {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			tg := f.target("main")
			require.NoError(t, tg.dir.mkdirs(caDir))
			user, check := f.unrelated()
			at, target := link(f, tg)
			require.NoError(t, f.m.Home.Root.RemoveAll(at))
			require.NoError(t, f.m.Home.Root.MkdirAll(filepath.Dir(at), 0o755))
			require.NoError(t, f.m.Home.Root.Symlink(target, at))
			info, err := user.Stat(filepath.Join(".crannog", at))
			require.NoError(t, err)
			require.True(t, info.IsDir(), "the symlink leads to the unrelated directory")

			_ = tg.dir.clear()
			_ = tg.dir.WriteBranch("main")
			_, _ = tg.dir.sessions(true)
			if l, err := tg.dir.lock(nil); err == nil {
				l.Unlock()
			}
			_ = tg.dir.removeAll()
			_, _ = f.m.List()
			f.m.SelfRepair()
			check()
		})
	}
}

// Symlinks within ~/.crannog are refused as well: a cleanup must not reach
// another crannog's state.
func TestHostDirRefusesSymlinksWithinTheHome(t *testing.T) {
	f := newFixture(t)
	other := f.target("other")
	require.NoError(t, other.dir.WriteBranch("other"))
	tg := f.target("main")
	require.NoError(t, f.m.Home.Root.Symlink(other.dir.slug, tg.dir.path))

	assert.ErrorContains(t, tg.dir.clear(), "not a directory")
	assert.ErrorContains(t, tg.dir.removeAll(), "not a directory")
	_, err := tg.dir.Branch()
	assert.ErrorContains(t, err, "not a directory")
	branch, err := other.dir.Branch()
	require.NoError(t, err)
	assert.Equal(t, "other", branch, "the other crannog's state is untouched")
}

// Self-repair takes slugs from remote URLs. One that is no branch slug must
// not lead out of the crannogs directory.
func TestSelfRepairIgnoresMalformedRemotes(t *testing.T) {
	f := newFixture(t)
	_, check := f.unrelated()
	slug := "../../../../unrelated"
	testgit.Git(t, f.dir, "config", "remote.crannog-"+slug+".url", "crannog::"+slug)

	f.m.SelfRepair()
	_, err := f.m.List()
	require.NoError(t, err)
	check()
	assert.Equal(t, "crannog::"+slug, testgit.Git(t, f.dir, "config", "remote.crannog-"+slug+".url"), "not crannog's remote")
}

// Entries of the crannogs directory that are not named like a branch slug
// are not crannog directories.
func TestListIgnoresOtherEntries(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.m.Home.Root.MkdirAll(filepath.Join(home.CrannogsPath(f.m.Repo.ID), "Not A Slug"), 0o755))
	testfs.WriteFile(t, f.m.Home.Root, filepath.Join(home.CrannogsPath(f.m.Repo.ID), "Not A Slug", branchFile), "x\n", 0o644)
	list, err := f.m.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}
