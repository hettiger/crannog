// Package config parses a setup's config.yml (ADR 0025).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Fixed paths in a crannog that config.yml cannot use as mount targets.
var reservedTargets = []string{"/work", "/state"}

// Config is the content of config.yml.
type Config struct {
	Session   Session           `yaml:"session"`
	Resources Resources         `yaml:"resources"`
	Proxy     Proxy             `yaml:"proxy"`
	Env       map[string]string `yaml:"env"`
	Mounts    []Mount           `yaml:"mounts"`
}

// Session configures what a session runs.
type Session struct {
	// Command is run by the agent user's login shell. It is not
	// interpolated; the shell in the crannog expands it.
	Command string `yaml:"command"`
}

// Resources are passed to container run as they are.
type Resources struct {
	CPUs   string `yaml:"cpus"`
	Memory string `yaml:"memory"`
}

// Proxy configures the proxy causeway.
type Proxy struct {
	Image string `yaml:"image"`
}

// Mount is a mount causeway.
type Mount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly"`
}

// Parse parses and validates config.yml. Unknown keys are an error.
func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("config.yml is empty")
		}
		return nil, fmt.Errorf("config.yml: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("config.yml: %w", err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	var errs []error
	required := map[string]string{
		"resources.cpus":   c.Resources.CPUs,
		"resources.memory": c.Resources.Memory,
		"proxy.image":      c.Proxy.Image,
	}
	for _, k := range sortedKeys(required) {
		if strings.TrimSpace(required[k]) == "" {
			errs = append(errs, fmt.Errorf("%s is required", k))
		}
	}
	for _, k := range sortedKeys(c.Env) {
		if !validName(k) {
			errs = append(errs, fmt.Errorf("env: invalid variable name %q", k))
		}
	}
	for i, m := range c.Mounts {
		if m.Source == "" || m.Target == "" {
			errs = append(errs, fmt.Errorf("mounts[%d]: source and target are required", i))
		}
	}
	for _, f := range c.interpolated() {
		if _, err := references(f.value); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.key, err))
		}
	}
	return errors.Join(errs...)
}

// field is an interpolated value and where it is in config.yml.
type field struct {
	key   string
	value string
	set   func(*Config, string)
}

// interpolated returns all values that are interpolated: everything except
// session.command.
func (c *Config) interpolated() []field {
	fs := []field{
		{"resources.cpus", c.Resources.CPUs, func(c *Config, v string) { c.Resources.CPUs = v }},
		{"resources.memory", c.Resources.Memory, func(c *Config, v string) { c.Resources.Memory = v }},
		{"proxy.image", c.Proxy.Image, func(c *Config, v string) { c.Proxy.Image = v }},
	}
	for _, k := range sortedKeys(c.Env) {
		fs = append(fs, field{"env." + k, c.Env[k], func(c *Config, v string) { c.Env[k] = v }})
	}
	for i, m := range c.Mounts {
		fs = append(fs,
			field{fmt.Sprintf("mounts[%d].source", i), m.Source, func(c *Config, v string) { c.Mounts[i].Source = v }},
			field{fmt.Sprintf("mounts[%d].target", i), m.Target, func(c *Config, v string) { c.Mounts[i].Target = v }},
		)
	}
	return fs
}

// HostVariables returns the sorted names of the variables the config reads
// from the host's environment.
func (c *Config) HostVariables() []string {
	seen := map[string]bool{}
	for _, f := range c.interpolated() {
		names, _ := references(f.value)
		for _, n := range names {
			if !strings.HasPrefix(n, Prefix) {
				seen[n] = true
			}
		}
	}
	return sortedKeys(seen)
}

// Expand returns a copy of the config with all variables interpolated and
// mount sources resolved: a source may start with ~/ for homeDir, relative
// sources are an error. Mount targets must be absolute and must not be
// /work, /state or below them.
func (c *Config) Expand(vars Vars, homeDir string) (*Config, error) {
	out := *c
	out.Env = make(map[string]string, len(c.Env))
	for k, v := range c.Env {
		out.Env[k] = v
	}
	out.Mounts = append([]Mount(nil), c.Mounts...)

	var errs []error
	for _, f := range c.interpolated() {
		v, err := Interpolate(f.value, vars)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.key, err))
			continue
		}
		f.set(&out, v)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("config.yml: %w", errors.Join(errs...))
	}

	for i := range out.Mounts {
		m := &out.Mounts[i]
		if rest, ok := strings.CutPrefix(m.Source, "~/"); ok {
			m.Source = filepath.Join(homeDir, rest)
		}
		if !filepath.IsAbs(m.Source) {
			errs = append(errs, fmt.Errorf("mounts[%d].source: %q is relative (use an absolute path or ~/)", i, m.Source))
		}
		if !path.IsAbs(m.Target) {
			errs = append(errs, fmt.Errorf("mounts[%d].target: %q is relative", i, m.Target))
			continue
		}
		m.Target = path.Clean(m.Target)
		for _, r := range reservedTargets {
			if m.Target == r || strings.HasPrefix(m.Target, r+"/") {
				errs = append(errs, fmt.Errorf("mounts[%d].target: %s is reserved for crannog's volume", i, r))
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("config.yml: %w", errors.Join(errs...))
	}
	return &out, nil
}

// EnvList returns the environment as sorted KEY=VALUE pairs.
func (c *Config) EnvList() []string {
	list := make([]string, 0, len(c.Env))
	for _, k := range sortedKeys(c.Env) {
		list = append(list, k+"="+c.Env[k])
	}
	return list
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
