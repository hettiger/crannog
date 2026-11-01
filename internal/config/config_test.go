package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func vars(host map[string]string) Vars {
	return Vars{
		Crannog: map[string]string{
			AgentsDir: "/Users/me/.crannog/agents",
			ProxyURL:  ProxyURLValue,
		},
		Host: func(n string) (string, bool) {
			v, ok := host[n]
			return v, ok
		},
	}
}

func TestInterpolate(t *testing.T) {
	v := vars(map[string]string{"TOKEN": "abc", "EMPTY": ""})
	tests := map[string]string{
		"plain":                       "plain",
		"$TOKEN":                      "abc",
		"${TOKEN}":                    "abc",
		"a${TOKEN}b":                  "aabcb",
		"$TOKEN-x":                    "abc-x",
		"$$TOKEN":                     "$TOKEN",
		"cost: $$5":                   "cost: $5",
		"${MISSING:-fallback}":        "fallback",
		"${EMPTY:-fallback}":          "fallback",
		"${TOKEN:-fallback}":          "abc",
		"${MISSING:-}":                "",
		"$EMPTY":                      "",
		"${CRANNOG_PROXY_URL}":        "http://crannog-proxy:8080",
		"$CRANNOG_AGENTS_DIR/claude":  "/Users/me/.crannog/agents/claude",
		"${MISSING:-a b:c/d}":         "a b:c/d",
		"${CRANNOG_PROXY_URL:-other}": "http://crannog-proxy:8080",
	}
	for in, want := range tests {
		got, err := Interpolate(in, v)
		if assert.NoError(t, err, in) {
			assert.Equal(t, want, got, in)
		}
	}
}

func TestInterpolateErrors(t *testing.T) {
	v := vars(nil)
	tests := map[string]string{
		"$MISSING":             "not set",
		"${MISSING}":           "not set",
		"$":                    "at the end",
		"a$":                   "at the end",
		"$1":                   "invalid use of $",
		"$-":                   "invalid use of $",
		"${":                   "missing }",
		"${}":                  "invalid variable",
		"${A-b}":               "invalid variable",
		"${A:?err}":            "invalid variable",
		"${A:-$B}":             "variables in defaults",
		"${CRANNOG_NAME}":      "unknown crannog variable",
		"${CRANNOG_STATE_DIR}": "unknown crannog variable",
	}
	for in, want := range tests {
		_, err := Interpolate(in, v)
		assert.ErrorContains(t, err, want, in)
	}
}

func TestInterpolateDoesNotOverrideCrannogVariables(t *testing.T) {
	v := vars(map[string]string{ProxyURL: "http://evil"})
	got, err := Interpolate("$CRANNOG_PROXY_URL", v)
	require.NoError(t, err)
	assert.Equal(t, ProxyURLValue, got)
}

const valid = `
session:
  command: claude --dangerously-skip-permissions "$HOME"
resources:
  cpus: 4
  memory: 4G
proxy:
  image: docker.io/ironsh/iron-proxy:0.54.0@sha256:abc
env:
  HTTPS_PROXY: ${CRANNOG_PROXY_URL}
  NODE_USE_ENV_PROXY: 1
  TOKEN: ${GH_TOKEN}
  OTHER: ${EDITOR:-vi}
mounts:
  - source: ${CRANNOG_AGENTS_DIR}
    target: /home/agent/.crannog/agents
  - source: ~/data
    target: /data/
    readonly: true
`

func TestParse(t *testing.T) {
	c, err := Parse([]byte(valid))
	require.NoError(t, err)
	assert.Equal(t, `claude --dangerously-skip-permissions "$HOME"`, c.Session.Command)
	assert.Equal(t, "4", c.Resources.CPUs)
	assert.Equal(t, "4G", c.Resources.Memory)
	assert.Equal(t, "1", c.Env["NODE_USE_ENV_PROXY"], "scalars are read as written")
	assert.Equal(t, []string{"EDITOR", "GH_TOKEN"}, c.HostVariables())
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"":                             "empty",
		"unknown: 1":                   "field unknown not found",
		"session:\n  cmd: x":           "field cmd not found",
		"resources: {cpus: 1}":         "resources.memory is required",
		"env: {A: $}":                  "env.A",
		"env: {\"A-B\": x}":            "invalid variable name",
		"mounts: [{source: /x}]":       "source and target are required",
		"mounts: [{target: /x, a: 1}]": "field a not found",
	}
	for in, want := range tests {
		_, err := Parse([]byte(in))
		assert.ErrorContains(t, err, want, in)
	}
}

func TestExpand(t *testing.T) {
	c, err := Parse([]byte(valid))
	require.NoError(t, err)

	_, err = c.Expand(vars(nil), "/Users/me")
	assert.ErrorContains(t, err, "GH_TOKEN is not set")

	e, err := c.Expand(vars(map[string]string{"GH_TOKEN": "t"}), "/Users/me")
	require.NoError(t, err)
	assert.Equal(t, "http://crannog-proxy:8080", e.Env["HTTPS_PROXY"])
	assert.Equal(t, "t", e.Env["TOKEN"])
	assert.Equal(t, "vi", e.Env["OTHER"])
	assert.Equal(t, `claude --dangerously-skip-permissions "$HOME"`, e.Session.Command, "not interpolated")
	assert.Equal(t, []Mount{
		{Source: "/Users/me/.crannog/agents", Target: "/home/agent/.crannog/agents"},
		{Source: "/Users/me/data", Target: "/data", ReadOnly: true},
	}, e.Mounts)
	assert.Equal(t, "${GH_TOKEN}", c.Env["TOKEN"], "original unchanged")
	assert.Equal(t, "~/data", c.Mounts[1].Source, "original unchanged")
	assert.Equal(t, []string{
		"HTTPS_PROXY=http://crannog-proxy:8080",
		"NODE_USE_ENV_PROXY=1",
		"OTHER=vi",
		"TOKEN=t",
	}, e.EnvList())
}

func TestExpandMountErrors(t *testing.T) {
	base := "resources: {cpus: 1, memory: 1G}\nproxy: {image: x}\nmounts:\n"
	tests := map[string]string{
		"  - {source: data, target: /data}":          "is relative (use an absolute path or ~/)",
		"  - {source: \"~\", target: /data}":         "is relative",
		"  - {source: /data, target: data}":          "target: \"data\" is relative",
		"  - {source: /data, target: /work}":         "reserved",
		"  - {source: /data, target: /state/x}":      "reserved",
		"  - {source: /data, target: /work/../work}": "reserved",
	}
	for m, want := range tests {
		c, err := Parse([]byte(base + m))
		require.NoError(t, err, m)
		_, err = c.Expand(vars(nil), "/Users/me")
		assert.ErrorContains(t, err, want, m)
	}
	c, err := Parse([]byte(base + "  - {source: /data, target: /workspace}"))
	require.NoError(t, err)
	_, err = c.Expand(vars(nil), "/Users/me")
	assert.NoError(t, err)
}
