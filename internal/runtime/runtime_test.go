package runtime

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hettiger/crannog/internal/testfs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shortened output of container list --all --format json, container 1.6.1.
const listJSON = `[
  {
    "configuration": {"id": "crannog-0123abcd-main-1a2b3c4d", "image": {"reference": "crannog-abc"}},
    "status": {
      "state": "running",
      "networks": [
        {"network": "crannog-0123abcd-main-1a2b3c4d-net", "ipv4Address": "192.168.66.3/24", "ipv4Gateway": "192.168.66.1", "hostname": "x"}
      ],
      "startedDate": "2026-11-01T10:00:00Z"
    }
  },
  {
    "configuration": {"id": "crannog-0123abcd-main-1a2b3c4d-proxy"},
    "status": {
      "state": "running",
      "networks": [
        {"network": "default", "ipv4Address": "192.168.64.5/24"},
        {"network": "crannog-0123abcd-main-1a2b3c4d-net", "ipv4Address": "192.168.66.2/24"}
      ]
    }
  },
  {
    "configuration": {"id": "other"},
    "status": {"state": "stopped", "networks": []}
  }
]`

func TestParseList(t *testing.T) {
	list, err := parseList([]byte(listJSON))
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "crannog-0123abcd-main-1a2b3c4d", list[0].Name)
	assert.Equal(t, StateRunning, list[0].State)
	assert.Equal(t, "192.168.66.2", list[1].IPv4On("crannog-0123abcd-main-1a2b3c4d-net"))
	assert.Equal(t, "192.168.64.5", list[1].IPv4On("default"))
	assert.Equal(t, "", list[2].IPv4On("default"))
	assert.Equal(t, StateStopped, list[2].State)

	_, err = parseList([]byte("not json"))
	assert.Error(t, err)
}

func TestSpecArgs(t *testing.T) {
	s := Spec{
		Name:     "c",
		Image:    "img",
		CPUs:     "4",
		Memory:   "4G",
		Networks: []string{"default", "net"},
		Mounts: []Mount{
			{Source: "c-work", Target: "/work"},
			{Source: "/Users/me/ca", Target: "/etc/crannog", ReadOnly: true},
		},
		Env:  []string{"A=1", "B=x y"},
		Args: []string{"-config", "/etc/iron-proxy/iron-proxy.yaml"},
	}
	assert.Equal(t, []string{
		"--name", "c", "--cpus", "4", "--memory", "4G",
		"--network", "default", "--network", "net",
		"--volume", "c-work:/work", "--volume", "/Users/me/ca:/etc/crannog:ro",
		"--env", "A=1", "--env", "B=x y",
		"img", "-config", "/etc/iron-proxy/iron-proxy.yaml",
	}, s.args())

	assert.Equal(t, []string{"--name", "x", "img"}, Spec{Name: "x", Image: "img"}.args())
	assert.Equal(t, []string{"img", "-config", "x"}, Spec{Image: "img", Args: []string{"-config", "x"}}.args())
}

func TestExecArgs(t *testing.T) {
	o := ExecOptions{User: "agent", Workdir: "/work/r", Interactive: true, TTY: true, Env: []string{"A=1"}}
	assert.Equal(t, []string{"--interactive", "--tty", "--user", "agent", "--workdir", "/work/r", "--env", "A=1"}, o.args())
	assert.Empty(t, ExecOptions{}.args())

	cmd := CLI{}.ExecCommand("c", ExecOptions{User: "root"}, "sh", "-c", "true")
	assert.Equal(t, []string{"container", "exec", "--user", "root", "c", "sh", "-c", "true"}, cmd.Args)
}

func TestCleanStderr(t *testing.T) {
	assert.Equal(t, "Error: not found; hint", cleanStderr("[0/6] [0s]\nError: not found\n\n hint \n"))
}

func TestReadinessCommandsCancelBlockedProcess(t *testing.T) {
	dir := t.TempDir()
	// exec keeps sleep in the CLI process, so cancellation must reap it.
	testfs.WriteFile(t, testfs.Root(t, dir), "container", "#!/bin/sh\necho 'probe started' >&2\nexec sleep 30\n", 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, command := range []string{"exec", "list"} {
		t.Run(command, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			started := time.Now()
			var err error
			if command == "exec" {
				_, err = (CLI{}).Exec(ctx, "test", ExecOptions{}, "true")
			} else {
				_, err = (CLI{}).List(ctx, "test")
			}
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.ErrorContains(t, err, "probe started")
			assert.Less(t, time.Since(started), 5*time.Second)
		})
	}
}
