// Package naming derives the global runtime names, the remote and its URL
// of a crannog (ADR 0012, ADR 0014, ADR 0021).
package naming

import "strings"

// URLPrefix is the prefix of a crannog remote's URL.
const URLPrefix = "crannog::"

// Names are the names of one crannog.
type Names struct {
	Slug        string
	Container   string
	Proxy       string
	Network     string
	WorkVolume  string
	StateVolume string
	Remote      string
	URL         string
}

// Prefix is the prefix of all runtime names of a repository's crannogs.
func Prefix(id8 string) string { return "crannog-" + id8 + "-" }

// New returns the names of the crannog with the given branch slug in the
// repository with the given id8.
func New(id8, slug string) Names {
	base := Prefix(id8) + slug
	return Names{
		Slug:        slug,
		Container:   base,
		Proxy:       base + "-proxy",
		Network:     base + "-net",
		WorkVolume:  base + "-work",
		StateVolume: base + "-state",
		Remote:      "crannog-" + slug,
		URL:         URLPrefix + slug,
	}
}

// SlugFromURL returns the branch slug of a crannog remote URL.
func SlugFromURL(url string) (string, bool) {
	slug, ok := strings.CutPrefix(url, URLPrefix)
	return slug, ok && slug != ""
}
