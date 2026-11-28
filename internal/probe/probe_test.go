package probe

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCP(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := netip.MustParseAddrPort(l.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NoError(t, TCP(ctx, addr), "something listens")

	require.NoError(t, l.Close())
	assert.Error(t, TCP(ctx, addr), "nothing listens anymore")
}
