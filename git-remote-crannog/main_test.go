package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServe(t *testing.T) {
	in := strings.NewReader("capabilities\nconnect git-upload-pack\n0032want ...")
	var out bytes.Buffer
	var service, rest string
	err := serve(in, &out, func(s string) error {
		service = s
		b, _ := io.ReadAll(in)
		rest = string(b)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "connect\n\n", out.String())
	assert.Equal(t, "git-upload-pack", service)
	assert.Equal(t, "0032want ...", rest, "the service gets the rest of stdin unread")
}

func TestServeReceivePack(t *testing.T) {
	var service string
	err := serve(strings.NewReader("connect git-receive-pack\n"), io.Discard, func(s string) error {
		service = s
		return errors.New("not running")
	})
	assert.EqualError(t, err, "not running")
	assert.Equal(t, "git-receive-pack", service)
}

func TestServeEnds(t *testing.T) {
	connect := func(string) error { t.Fatal("unexpected connect"); return nil }
	assert.NoError(t, serve(strings.NewReader("capabilities\n\n"), io.Discard, connect))
	assert.NoError(t, serve(strings.NewReader(""), io.Discard, connect))
	assert.ErrorContains(t, serve(strings.NewReader("list\n"), io.Discard, connect), "unsupported command: list")
}
