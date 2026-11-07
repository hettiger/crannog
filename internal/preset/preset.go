// Package preset provides the bundled presets and the user's presets in
// ~/.crannog/presets/<name>/ (ADR 0015).
package preset

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/hostfs"
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
	// Dir is the absolute path of a user preset, for messages.
	Dir string
	// home is ~/.crannog, for reading a user preset.
	home home.Home
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
func List(h home.Home) ([]Preset, error) {
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

	entries, err = userPresets(h)
	if err != nil {
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
			Dir:             h.Abs(home.Presets + "/" + e.Name()),
			home:            h,
		}
	}

	list := make([]Preset, 0, len(byName))
	for _, p := range byName {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// userPresets lists ~/.crannog/presets/.
func userPresets(h home.Home) ([]fs.DirEntry, error) {
	r, err := h.OpenDir(home.Presets, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return fs.ReadDir(r.FS(), ".")
}

// Find returns the preset with the given name.
func Find(h home.Home, name string) (Preset, error) {
	list, err := List(h)
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

// Publish writes a setup to .crannog/ in the worktree at dir, replacing an
// existing one. The new content is written next to it first, so a failure
// leaves the old .crannog/ in place. Only .crannog/ and its temporary
// sibling are accessed in the worktree (ADR 0032).
func Publish(s setup.Setup, dir string) error {
	wt, err := hostfs.OpenAnchor(dir, false)
	if err != nil {
		return err
	}
	defer wt.Close()
	tmp, err := hostfs.MkdirTemp(wt, setup.Dir+".tmp-")
	if err != nil {
		return err
	}
	defer wt.RemoveAll(tmp)
	if err := s.Write(wt, tmp+"/"+setup.Dir); err != nil {
		return err
	}
	if err := wt.RemoveAll(setup.Dir); err != nil {
		return err
	}
	return wt.Rename(tmp+"/"+setup.Dir, setup.Dir)
}

// Load reads the content of a preset and checks that it is a complete
// setup.
func (p Preset) Load() (setup.Setup, error) {
	var s setup.Setup
	var err error
	if p.User {
		s, err = setup.ReadDir(p.home.Root, home.Presets+"/"+p.Name)
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
