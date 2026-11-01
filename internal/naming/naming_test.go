package naming

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hettiger/crannog/internal/slug"
)

func TestNew(t *testing.T) {
	n := New("0123abcd", "feature-x-217d2bf5")
	assert.Equal(t, Names{
		Slug:        "feature-x-217d2bf5",
		Container:   "crannog-0123abcd-feature-x-217d2bf5",
		Proxy:       "crannog-0123abcd-feature-x-217d2bf5-proxy",
		Network:     "crannog-0123abcd-feature-x-217d2bf5-net",
		WorkVolume:  "crannog-0123abcd-feature-x-217d2bf5-work",
		StateVolume: "crannog-0123abcd-feature-x-217d2bf5-state",
		Remote:      "crannog-feature-x-217d2bf5",
		URL:         "crannog::feature-x-217d2bf5",
	}, n)
	assert.True(t, strings.HasPrefix(n.Proxy, Prefix("0123abcd")))
}

// Runtime names stay within the 63 characters of container 1.6.1.
func TestLongestName(t *testing.T) {
	n := New("0123abcd", slug.BranchSlug(strings.Repeat("x", 200)))
	assert.Equal(t, 63, len(n.Proxy))
}

func TestSlugFromURL(t *testing.T) {
	s, ok := SlugFromURL("crannog::main-1a2b3c4d")
	assert.True(t, ok)
	assert.Equal(t, "main-1a2b3c4d", s)
	_, ok = SlugFromURL("crannog::")
	assert.False(t, ok)
	_, ok = SlugFromURL("git@github.com:x/y")
	assert.False(t, ok)
}
