package seatbelt

import (
	_ "embed"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/testfs"
)

var locations = hostfs.Locations{
	Home:        "/Users/u",
	CrannogHome: "/Users/u/.crannog",
	TempDir:     "/private/var/folders/x/T",
	Worktree:    "/Users/u/code/app",
	GitDir:      "/Users/u/code/app/.git",
}

func TestCommandRunsTheBinaryThroughSandboxExec(t *testing.T) {
	argv := command("/opt/bin/crannog", []string{"checkout", "--", "-p", "x"}, locations)

	assert.Equal(t, []string{
		"/usr/bin/sandbox-exec", "-p", profile,
		"-D", "CRANNOG_HOME=/Users/u/.crannog",
		"-D", "TEMP_DIR=/private/var/folders/x/T",
		"-D", "WORKTREE=/Users/u/code/app",
		"-D", "GIT_DIR=/Users/u/code/app/.git",
		"-D", "BUILDER_DIR=/Users/u/Library/Application Support/com.apple.container/builder",
		"/opt/bin/crannog", "checkout", "--", "-p", "x",
	}, argv)
}

func TestCommandOutsideOfARepositoryAllowsNothingForIt(t *testing.T) {
	l := locations
	l.Worktree, l.GitDir = "", ""

	argv := command("/opt/bin/crannog", nil, l)

	assert.Contains(t, argv, "WORKTREE=/dev/null")
	assert.Contains(t, argv, "GIT_DIR=/dev/null")
}

// Every parameter the profile uses gets a value, and no other is passed:
// sandbox-exec fails on a missing one.
func TestProfileParametersMatchTheCommand(t *testing.T) {
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`\(param "([A-Z_]+)"\)`).FindAllStringSubmatch(profile, -1) {
		used[m[1]] = true
	}
	passed := map[string]bool{}
	argv := command("/opt/bin/crannog", nil, locations)
	for i, a := range argv {
		if a == "-D" {
			name, _, _ := strings.Cut(argv[i+1], "=")
			passed[name] = true
		}
	}

	assert.NotEmpty(t, used)
	assert.Equal(t, used, passed)
}

//go:embed test.sb
var testProfile string

// The profiles deny all writes first and name no path of their own
// outside of /dev.
func TestProfilesAllowWritesOnlyInParametersAndDevices(t *testing.T) {
	for name, p := range map[string]string{"profile.sb": profile, "test.sb": testProfile} {
		code := regexp.MustCompile(`(?m);;.*$`).ReplaceAllString(p, "")

		assert.Less(t, strings.Index(code, "(deny file-write*)"), strings.Index(code, "(allow file-write*"), name)
		assert.Equal(t, 1, strings.Count(code, "(allow file-write*"), name)
		for _, m := range regexp.MustCompile(`\((literal|subpath|regex) #?"([^"]*)"`).FindAllStringSubmatch(code, -1) {
			assert.Regexp(t, `^\^?/dev/`, m[2], name)
		}
	}
}

// The tests' profile allows less than the binaries' one: no crannog
// home, no worktree, no git directory.
func TestTestProfileParameters(t *testing.T) {
	assert.Equal(t, []string{"TEMP_DIR", "GO_CACHE"}, parameters(testProfile))
}

// The Makefile passes exactly the parameters of the tests' profile.
func TestMakefilePassesTheParametersOfTheTestProfile(t *testing.T) {
	makefile, err := fs.ReadFile(testfs.Repo(t), "Makefile")
	require.NoError(t, err)
	require.Contains(t, string(makefile), "/usr/bin/sandbox-exec -f internal/seatbelt/test.sb")
	var passed []string
	for _, m := range regexp.MustCompile(`-D "([A-Z_]+)=`).FindAllStringSubmatch(string(makefile), -1) {
		passed = append(passed, m[1])
	}

	assert.Equal(t, parameters(testProfile), passed)
}

// parameters returns the names of the parameters a profile uses.
func parameters(profile string) []string {
	var names []string
	for _, m := range regexp.MustCompile(`\(param "([A-Z_]+)"\)`).FindAllStringSubmatch(profile, -1) {
		names = append(names, m[1])
	}
	return names
}

// The rule for the worktree matches what hostfs creates there: the setup
// directory and its temporary siblings, and nothing else.
func TestProfileWorktreeRuleMatchesTheSetupDirectoryAndItsSiblings(t *testing.T) {
	m := regexp.MustCompile(`\(regex-quote \(param "WORKTREE"\)\) "((?:[^"\\]|\\.)*)"`).FindStringSubmatch(profile)
	require.NotNil(t, m)
	// In the profile the pattern is a string: \\ stands for one backslash.
	suffix := strings.ReplaceAll(m[1], `\\`, `\`)

	root, dir := testfs.TempRoot(t)
	require.NoError(t, root.Mkdir(".git", 0o755))
	wt, err := hostfs.OpenWorktree(dir)
	require.NoError(t, err)
	defer wt.Close()
	tmp, err := wt.NewSetupTemp()
	require.NoError(t, err)
	defer tmp.Remove()
	rule := regexp.MustCompile("^" + regexp.QuoteMeta(dir) + suffix)

	for _, name := range []string{hostfs.SetupDir, filepath.Join(hostfs.SetupDir, "config.yml")} {
		assert.True(t, rule.MatchString(filepath.Join(dir, name)), name)
	}
	assert.True(t, rule.MatchString(tmp.Dir), tmp.Dir)
	assert.True(t, rule.MatchString(filepath.Join(tmp.Dir, "old", "config.yml")))
	for _, name := range []string{"README", ".crannogX", ".crannog.tmp-evil", ".crannog.tmp-", ".git", filepath.Join("sub", hostfs.SetupDir)} {
		assert.False(t, rule.MatchString(filepath.Join(dir, name)), name)
	}
	assert.False(t, rule.MatchString(dir))
}
