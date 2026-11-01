// Package preset provides the bundled presets and the user's presets in
// ~/.crannog/presets/<name>/ (ADR 0015).
package preset

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hettiger/crannog/internal/setup"
)

// Default is the preset used for repositories without .crannog/.
const Default = "default"

//go:embed all:bundled
var bundled embed.FS

// Preset is a named template for .crannog/.
type Preset struct {
	Name string
	// User is true for presets in ~/.crannog/presets/.
	User bool
	// ReplacesBundled is true for a user preset with the name of a bundled
	// one, which it replaces completely.
	ReplacesBundled bool
	// Dir is the directory of a user preset.
	Dir string
}

// Source describes where the preset comes from, e.g. for crannog's
// output.
func (p Preset) Source() string {
	switch {
	case p.ReplacesBundled:
		return "user, replaces bundled"
	case p.User:
		return "user"
	default:
		return "bundled"
	}
}

// List returns all presets, sorted by name. A user preset hides a bundled
// one of the same name.
func List(presetsDir string) ([]Preset, error) {
	byName := map[string]Preset{}
	entries, err := fs.ReadDir(bundled, "bundled")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			byName[e.Name()] = Preset{Name: e.Name()}
		}
	}

	entries, err = os.ReadDir(presetsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		// Symlinks are not followed (and not listed).
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		_, isBundled := byName[e.Name()]
		byName[e.Name()] = Preset{
			Name:            e.Name(),
			User:            true,
			ReplacesBundled: isBundled,
			Dir:             filepath.Join(presetsDir, e.Name()),
		}
	}

	list := make([]Preset, 0, len(byName))
	for _, p := range byName {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// Find returns the preset with the given name.
func Find(presetsDir, name string) (Preset, error) {
	list, err := List(presetsDir)
	if err != nil {
		return Preset{}, err
	}
	for _, p := range list {
		if p.Name == name {
			return p, nil
		}
	}
	return Preset{}, fmt.Errorf("no preset named %q (see crannog preset list)", name)
}

// Publish writes a setup to .crannog/ in dir, replacing an existing one.
// The new content is written next to it first, so a failure leaves the old
// .crannog/ in place.
func Publish(s setup.Setup, dir string) error {
	tmp, err := os.MkdirTemp(dir, setup.Dir+".tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := s.Write(filepath.Join(tmp, setup.Dir)); err != nil {
		return err
	}
	target := filepath.Join(dir, setup.Dir)
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(filepath.Join(tmp, setup.Dir), target)
}

// Load reads the content of a preset and checks that it is a complete
// setup.
func (p Preset) Load() (setup.Setup, error) {
	var s setup.Setup
	var err error
	if p.User {
		s, err = setup.ReadDir(p.Dir)
	} else {
		var sub fs.FS
		sub, err = fs.Sub(bundled, "bundled/"+p.Name)
		if err == nil {
			s, err = setup.ReadFS(sub)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("preset %s: %w", p.Name, err)
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("preset %s: %w", p.Name, err)
	}
	return s, nil
}
