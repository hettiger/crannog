package slug

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchSlug(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		// Examples from ADR 0021 and ADR 0026.
		{"feature/x", "feature-x-217d2bf5"},
		{"feature-x", "feature-x-c791eb83"},
		{"feature/JIRA-1234-implement-the-new-billing-export", "feature-jira-1234-implement-the-41ed7bc7"},
		{"ü", "607474ca"},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			assert.Equal(t, tt.want, BranchSlug(tt.branch))
		})
	}
}

func TestClean(t *testing.T) {
	tests := map[string]string{
		"feature/Foo_bar": "feature-foo-bar", // ADR 0012
		"main":            "main",
		"--a//b--":        "a-b",
		"Ünïcode-ok":      "n-code-ok",
		"a.b@{c}":         "a-b-c",
		"äöü":             "",
		"UPPER":           "upper",
	}
	for in, want := range tests {
		assert.Equal(t, want, clean(in), in)
	}
}

func TestBranchSlugShape(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	branches := []string{
		"main",
		strings.Repeat("a", 100),
		strings.Repeat("ab-", 20),
		"x/" + strings.Repeat("-", 40) + "y",
		strings.Repeat("1", 30) + "/xyz", // cut ends with a dash
		"ü/ö",
	}
	for _, b := range branches {
		s := BranchSlug(b)
		assert.LessOrEqual(t, len(s), 40, b)
		assert.Regexp(t, valid, s, b)
	}
	assert.Regexp(t, `^1{30}-[0-9a-f]{8}$`, BranchSlug(strings.Repeat("1", 30)+"/xyz"))
}
