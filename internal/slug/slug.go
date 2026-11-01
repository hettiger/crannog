// Package slug derives branch slugs from start branches (ADR 0012, 0021, 0026).
package slug

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// maxCleaned is the number of characters of the cleaned branch kept in a
// branch slug. With "-" and the hash, a slug has at most 40 characters.
const maxCleaned = 31

// BranchSlug returns the branch slug of a start branch: the branch cleaned
// to lowercase kebab-case, cut to 31 characters, followed by "-" and the
// first eight hex characters of the branch's sha256. If the cleaned branch
// is empty, the slug is the hash alone.
func BranchSlug(branch string) string {
	sum := sha256.Sum256([]byte(branch))
	hash := hex.EncodeToString(sum[:])[:8]

	cleaned := clean(branch)
	if len(cleaned) > maxCleaned {
		cleaned = strings.TrimRight(cleaned[:maxCleaned], "-")
	}
	if cleaned == "" {
		return hash
	}
	return cleaned + "-" + hash
}

// clean lowercases ASCII letters, replaces every other character outside
// a-z0-9 with "-", collapses repeated dashes and trims them at both ends.
func clean(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			r += 'a' - 'A'
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		default:
			r = '-'
		}
		if r == '-' {
			if dash {
				continue
			}
			dash = true
		} else {
			dash = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "-")
}
