// git-remote-crannog is the git remote helper of the git causeway
// (ADR 0010). git runs it for remotes with the URL crannog::<slug>. It
// supports only the connect capability and pipes git-upload-pack and
// git-receive-pack through container exec. It never starts a crannog
// (ADR 0014).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-remote-crannog:", err)
		os.Exit(1)
	}
}

func run() error {
	// git passes the remote (name or URL) and the address after crannog::.
	if len(os.Args) != 3 || os.Args[2] == "" {
		return errors.New("usage: git-remote-crannog <remote> <slug> (run by git for crannog::<slug> URLs)")
	}
	slug := os.Args[2]
	return serve(os.Stdin, os.Stdout, func(service string) error {
		return connect(slug, service)
	})
}

// serve speaks the remote helper protocol on r and w. connect replaces the
// process with the service, so it only returns on errors.
func serve(r io.Reader, w io.Writer, connect func(service string) error) error {
	for {
		line, err := readLine(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch line {
		case "capabilities":
			if _, err := io.WriteString(w, "connect\n\n"); err != nil {
				return err
			}
		case "connect git-upload-pack", "connect git-receive-pack":
			return connect(line[len("connect "):])
		case "":
			return nil
		default:
			return fmt.Errorf("unsupported command: %s", line)
		}
	}
}

// readLine reads one line byte by byte: after the connect command, the
// rest of stdin belongs to the service and must not be buffered here.
func readLine(r io.Reader) (string, error) {
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				return string(line), nil
			}
			line = append(line, b[0])
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}
	}
}

// connect checks that the crannog runs, confirms the connection to git and
// replaces this process with the service in the crannog.
func connect(slug, service string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo, err := repository.Locate(cwd)
	if err != nil {
		return err
	}
	n := naming.New(repo.ID8(), slug)
	list, err := runtime.CLI{}.List(n.Container)
	if err != nil {
		return err
	}
	running := false
	for _, c := range list {
		if c.Name == n.Container && c.State == runtime.StateRunning {
			running = true
		}
	}
	if !running {
		return fmt.Errorf("the crannog %s is not running; it runs while a session does (crannog checkout)", slug)
	}

	o := runtime.ExecOptions{User: "agent", Interactive: true}
	if p := os.Getenv("GIT_PROTOCOL"); p != "" {
		o.Env = []string{"GIT_PROTOCOL=" + p}
	}
	cmd := runtime.CLI{}.ExecCommand(n.Container, o, service, repo.PathInCrannog())
	path, err := exec.LookPath(cmd.Path)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.WriteString("\n"); err != nil {
		return err
	}
	return syscall.Exec(path, cmd.Args, os.Environ())
}
