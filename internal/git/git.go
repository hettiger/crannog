// Package git runs the user's git on the host. It is the only place that
// starts git and has one function per operation crannog needs: no function
// takes free arguments for git (ADR 0031).
package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/slug"
)

// helper is the remote helper git runs for crannog:: URLs (ADR 0015).
const helper = "git-remote-crannog"

// command returns a git command that runs in dir.
func command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd
}

// run runs git in dir and returns its stdout without the trailing newline.
// On failure, the error contains git's stderr.
func run(dir string, args ...string) (string, error) {
	out, err := output(dir, args...)
	return strings.TrimRight(string(out), "\n"), err
}

// output runs git in dir and returns its stdout unchanged.
func output(dir string, args ...string) ([]byte, error) {
	cmd := command(dir, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return out, nil
}

// Error is a failed git command.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// exitCode returns the exit code of a failed git command, or -1.
func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// argument refuses a value that git would read as an option where it
// becomes an argument of its own.
func argument(what, v string) error {
	if v == "" || strings.HasPrefix(v, "-") {
		return fmt.Errorf("git: invalid %s %q", what, v)
	}
	return nil
}

// crannogRemote refuses every remote name but crannog-<slug> and returns
// the slug: other remotes are the user's (ADR 0015).
func crannogRemote(name string) (string, error) {
	s, ok := strings.CutPrefix(name, naming.RemotePrefix)
	if !ok || !slug.Valid(s) {
		return "", fmt.Errorf("git: %q is not the remote of a crannog", name)
	}
	return s, nil
}

// crannogURL refuses every URL but crannog::<slug> and returns the slug.
func crannogURL(url string) (string, error) {
	s, ok := naming.SlugFromURL(url)
	if !ok {
		return "", fmt.Errorf("git: %q is not the URL of a crannog", url)
	}
	return s, nil
}

// crannogRemoteWithURL refuses a remote and a URL that do not belong to
// the same crannog.
func crannogRemoteWithURL(name, url string) error {
	s, err := crannogRemote(name)
	if err != nil {
		return err
	}
	u, err := crannogURL(url)
	if err != nil {
		return err
	}
	if s != u {
		return fmt.Errorf("git: the remote %s and the URL %s belong to different crannogs", name, url)
	}
	return nil
}

// CheckHelper fails if git cannot find the remote helper.
func CheckHelper() error {
	if _, err := exec.LookPath(helper); err != nil {
		return errors.New(helper + " is not in PATH; install it next to crannog")
	}
	return nil
}

// Reading.

// TopLevel returns the top level of the worktree that contains dir.
func TopLevel(dir string) (string, error) {
	return run(dir, "rev-parse", "--show-toplevel")
}

// CommonDir returns the absolute path of the git common dir of the
// repository at dir.
func CommonDir(dir string) (string, error) {
	return run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// Worktrees returns the output of git worktree list --porcelain.
func Worktrees(dir string) (string, error) {
	return run(dir, "worktree", "list", "--porcelain")
}

// CurrentBranch returns the branch checked out in the worktree at dir, or
// "" on a detached HEAD.
func CurrentBranch(dir string) (string, error) {
	b, err := run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if exitCode(err) == 1 {
		return "", nil
	}
	return b, err
}

// HasBranch reports whether branch is a local branch. show-ref takes the
// name literally; rev-parse would also accept revision expressions such
// as main^ or main@{1}.
func HasBranch(dir, branch string) (bool, error) {
	_, err := run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if exitCode(err) == 1 {
		return false, nil
	}
	return err == nil, err
}

// BranchCommit returns the abbreviated hash of the commit of a local
// branch.
func BranchCommit(dir, branch string) (string, error) {
	return run(dir, "rev-parse", "--short", "refs/heads/"+branch)
}

// Config returns the value of a configuration key and whether it is set.
func Config(dir, key string) (string, bool, error) {
	if err := argument("configuration key", key); err != nil {
		return "", false, err
	}
	v, err := run(dir, "config", "--get", key)
	if exitCode(err) == 1 {
		return "", false, nil
	}
	return v, err == nil, err
}

// RemoteURL returns the URL of a remote named like a crannog's, or "" if
// there is none.
func RemoteURL(dir, name string) (string, error) {
	if _, err := crannogRemote(name); err != nil {
		return "", err
	}
	url, _, err := Config(dir, "remote."+name+".url")
	return url, err
}

// RemoteURLs returns the URLs of all remotes whose names start like a
// crannog's, by name. They can be the user's.
func RemoteURLs(dir string) (map[string]string, error) {
	out, err := run(dir, "config", "--get-regexp", `^remote\.`+naming.RemotePrefix+`.*\.url$`)
	if exitCode(err) == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	urls := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		key, url, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		urls[strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")] = url
	}
	return urls, nil
}

// Files returns the files below a path that are tracked, or untracked and
// not ignored, in the worktree at dir, each followed by a NUL.
func Files(dir, pathspec string) ([]byte, error) {
	return output(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", pathspec)
}

// Tree returns the entries below a path in a revision as git ls-tree -r
// prints them, each followed by a NUL.
func Tree(dir, rev, pathspec string) ([]byte, error) {
	if err := argument("revision", rev); err != nil {
		return nil, err
	}
	return output(dir, "ls-tree", "-r", "-z", "--full-tree", rev, "--", pathspec)
}

// Blob returns the content of the blob with the given object id.
func Blob(dir, object string) ([]byte, error) {
	if object == "" || strings.Trim(object, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("git: invalid object id %q", object)
	}
	return output(dir, "cat-file", "blob", object)
}

// Status returns the uncommitted changes below a path in the worktree at
// dir, untracked files included, as git status --porcelain prints them.
func Status(dir, pathspec string) (string, error) {
	return run(dir, "status", "--porcelain", "--untracked-files=all", "--", pathspec)
}

// DiffDirs writes the difference between the directories a and b in dir
// to out. It compares files only and needs no repository. With pager, git
// may show the difference in the user's pager.
func DiffDirs(dir, a, b string, out io.Writer, pager bool) error {
	args := []string{"-c", "core.quotepath=false"}
	if !pager {
		args = append(args, "--no-pager")
	}
	args = append(args, "diff", "--no-index", "--no-prefix", "--", a, b)
	cmd := command(dir, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	// git diff exits with 1 if there are differences.
	if err := cmd.Run(); err != nil && exitCode(err) != 1 {
		return fmt.Errorf("git diff: %w", err)
	}
	return nil
}

// Writing. These functions change the user's repository or send it
// somewhere. They accept only a crannog's remote and URL.

// AddRemote adds the remote of a crannog and keeps git from following the
// crannog's HEAD.
func AddRemote(dir, name, url string) error {
	if err := crannogRemoteWithURL(name, url); err != nil {
		return err
	}
	if _, err := run(dir, "remote", "add", name, url); err != nil {
		return err
	}
	_, err := run(dir, "config", "remote."+name+".followRemoteHEAD", "never")
	return err
}

// RemoveRemote removes the remote of a crannog with its remote-tracking
// branches. A remote of that name with another URL is the user's and
// stays; one with other fetch mappings than git remote add writes is an
// error, as removing it would delete the refs they name.
func RemoveRemote(dir, name, url string) error {
	if err := crannogRemoteWithURL(name, url); err != nil {
		return err
	}
	current, err := RemoteURL(dir, name)
	if err != nil || current != url {
		return err
	}
	mappings, err := run(dir, "config", "--get-all", "remote."+name+".fetch")
	if err != nil && exitCode(err) != 1 {
		return err
	}
	if err != nil || mappings != "+refs/heads/*:refs/remotes/"+name+"/*" {
		return fmt.Errorf("refusing to remove remote %s: unexpected fetch mappings", name)
	}
	_, err = run(dir, "remote", "remove", name)
	return err
}

// PushToCrannog pushes the repository to a new crannog: all branches as
// origin/*, all tags, and the start branch as a local branch, like in a
// fresh clone (ADR 0015). It pushes to the URL, not to a configured
// remote, and never with force. git's output goes to out.
func PushToCrannog(dir, url, branch string, out io.Writer) error {
	if _, err := crannogURL(url); err != nil {
		return err
	}
	if branch == "" {
		return errors.New("git: no branch to push")
	}
	cmd := command(dir, "push", "--quiet", url,
		"refs/heads/*:refs/remotes/origin/*",
		"refs/tags/*:refs/tags/*",
		"refs/heads/"+branch+":refs/heads/"+branch)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
