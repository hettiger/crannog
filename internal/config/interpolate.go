package config

import (
	"fmt"
	"strings"
)

// Prefix of the variables crannog provides. They cannot be overridden.
const Prefix = "CRANNOG_"

// Names of the variables crannog provides (ADR 0019).
const (
	AgentsDir = "CRANNOG_AGENTS_DIR"
	CacheDir  = "CRANNOG_CACHE_DIR"
	CADir     = "CRANNOG_CA_DIR"
	RepoDir   = "CRANNOG_REPO_DIR"
	ProxyURL  = "CRANNOG_PROXY_URL"
)

// ProxyURLValue is the value of CRANNOG_PROXY_URL (ADR 0020).
const ProxyURLValue = "http://crannog-proxy:8080"

// Vars are the variables available to interpolation.
type Vars struct {
	// Crannog holds the CRANNOG_* variables.
	Crannog map[string]string
	// Host looks up a variable in the environment crannog is called in.
	Host func(name string) (string, bool)
}

// ref is a variable reference in a value.
type ref struct {
	name       string
	hasDefault bool
	def        string
}

// part is a literal or a reference.
type part struct {
	literal string
	ref     *ref
}

// parse splits a value into literals and references, like Docker Compose:
// $VAR, ${VAR}, ${VAR:-default} and $$ for a literal $. Any other $ is an
// error.
func parse(s string) ([]part, error) {
	var parts []part
	var lit strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			lit.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			return nil, fmt.Errorf("%q: $ at the end (write $$ for a literal $)", s)
		}
		switch c := s[i+1]; {
		case c == '$':
			lit.WriteByte('$')
			i++
			continue
		case c == '{':
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				return nil, fmt.Errorf("%q: missing } after ${", s)
			}
			inner := s[i+2 : i+2+end]
			r, err := parseBraced(inner)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", s, err)
			}
			parts = flush(parts, &lit)
			parts = append(parts, part{ref: r})
			i += 2 + end
		case isNameStart(c):
			j := i + 1
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			parts = flush(parts, &lit)
			parts = append(parts, part{ref: &ref{name: s[i+1 : j]}})
			i = j - 1
		default:
			return nil, fmt.Errorf("%q: invalid use of $ (write $$ for a literal $)", s)
		}
	}
	return flush(parts, &lit), nil
}

func parseBraced(inner string) (*ref, error) {
	name, def, hasDefault := strings.Cut(inner, ":-")
	if !validName(name) {
		return nil, fmt.Errorf("invalid variable ${%s} (supported: $VAR, ${VAR}, ${VAR:-default})", inner)
	}
	if strings.Contains(def, "$") {
		return nil, fmt.Errorf("variables in defaults are not supported: ${%s}", inner)
	}
	return &ref{name: name, hasDefault: hasDefault, def: def}, nil
}

func flush(parts []part, lit *strings.Builder) []part {
	if lit.Len() > 0 {
		parts = append(parts, part{literal: lit.String()})
		lit.Reset()
	}
	return parts
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameChar(c byte) bool { return isNameStart(c) || (c >= '0' && c <= '9') }

func validName(s string) bool {
	if s == "" || !isNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isNameChar(s[i]) {
			return false
		}
	}
	return true
}

// Interpolate expands the variables in s. CRANNOG_* variables come from
// crannog; all others from the host. Unlike Compose, an unset variable
// without a default is an error. ${VAR:-default} uses the default if VAR
// is unset or empty.
func Interpolate(s string, vars Vars) (string, error) {
	parts, err := parse(s)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range parts {
		if p.ref == nil {
			b.WriteString(p.literal)
			continue
		}
		v, ok, err := lookup(p.ref.name, vars)
		if err != nil {
			return "", err
		}
		switch {
		case p.ref.hasDefault && (!ok || v == ""):
			v = p.ref.def
		case !ok:
			return "", fmt.Errorf("variable %s is not set (use ${%s:-default} for a default)", p.ref.name, p.ref.name)
		}
		b.WriteString(v)
	}
	return b.String(), nil
}

func lookup(name string, vars Vars) (string, bool, error) {
	if strings.HasPrefix(name, Prefix) {
		v, ok := vars.Crannog[name]
		if !ok {
			return "", false, fmt.Errorf("unknown crannog variable %s", name)
		}
		return v, true, nil
	}
	if vars.Host == nil {
		return "", false, nil
	}
	v, ok := vars.Host(name)
	return v, ok, nil
}

// references returns the names of the variables s refers to.
func references(s string) ([]string, error) {
	parts, err := parse(s)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range parts {
		if p.ref != nil {
			names = append(names, p.ref.name)
		}
	}
	return names, nil
}
