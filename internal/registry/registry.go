// Package registry describes which Claude Code theme tokens exist, where that
// knowledge comes from, and the default palette of each base preset.
//
// The data is generated at development time from the official documentation
// (see tools/genregistry) and embedded; nothing is fetched at runtime.
package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Status says how trustworthy a token is.
type Status string

const (
	// Verified tokens appear in the official Claude Code documentation.
	Verified Status = "verified"
	// Unverified tokens are accepted by a captured Claude Code version but are
	// not documented. They are preserved but never offered in the editor.
	Unverified Status = "unverified"
)

// Role tells the preview and contrast checks how a token is painted.
type Role string

const (
	RoleForeground Role = "foreground"
	RoleBackground Role = "background"
)

// Token is the metadata of one theme token.
type Token struct {
	Key            string `json:"key"`
	DisplayName    string `json:"displayName"`
	Category       string `json:"category"`
	Role           Role   `json:"role"`
	Description    string `json:"description"`
	Status         Status `json:"status"`
	Source         string `json:"source"`
	SinceVersion   string `json:"sinceVersion,omitempty"`
	FullscreenOnly bool   `json:"fullscreenOnly,omitempty"`
}

// Registry is the immutable token catalogue plus base palettes.
type Registry struct {
	Version         string
	DocsURL         string
	VerifiedAgainst string
	PaletteSource   string
	PaletteNote     string

	tokens   []Token
	byKey    map[string]Token
	palettes map[theme.Base]map[string]theme.Color
}

//go:embed data/tokens.json
var tokensJSON []byte

//go:embed data/palettes.json
var palettesJSON []byte

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

// Default returns the embedded registry. The embedded data is covered by
// tests, so an error here means a broken build.
func Default() (*Registry, error) {
	defaultOnce.Do(func() { defaultReg, defaultErr = Load(tokensJSON, palettesJSON) })
	return defaultReg, defaultErr
}

type tokensFile struct {
	RegistryVersion string  `json:"registryVersion"`
	DocsURL         string  `json:"docsUrl"`
	VerifiedAgainst string  `json:"verifiedAgainst"`
	Tokens          []Token `json:"tokens"`
}

type palettesFile struct {
	CapturedFrom string                       `json:"capturedFrom"`
	Note         string                       `json:"note"`
	Palettes     map[string]map[string]string `json:"palettes"`
}

// Load parses registry data. Exposed for tests and future external registries.
func Load(tokensData, palettesData []byte) (*Registry, error) {
	var tf tokensFile
	if err := json.Unmarshal(tokensData, &tf); err != nil {
		return nil, fmt.Errorf("registry tokens: %w", err)
	}
	var pf palettesFile
	if err := json.Unmarshal(palettesData, &pf); err != nil {
		return nil, fmt.Errorf("registry palettes: %w", err)
	}
	r := &Registry{
		Version: tf.RegistryVersion, DocsURL: tf.DocsURL, VerifiedAgainst: tf.VerifiedAgainst,
		PaletteSource: pf.CapturedFrom, PaletteNote: pf.Note,
		tokens: tf.Tokens, byKey: make(map[string]Token, len(tf.Tokens)),
		palettes: make(map[theme.Base]map[string]theme.Color, len(pf.Palettes)),
	}
	for _, t := range tf.Tokens {
		if t.Status != Verified && t.Status != Unverified {
			return nil, fmt.Errorf("registry token %q: unknown status %q", t.Key, t.Status)
		}
		if _, dup := r.byKey[t.Key]; dup {
			return nil, fmt.Errorf("registry token %q: duplicate", t.Key)
		}
		r.byKey[t.Key] = t
	}
	for _, b := range theme.Bases {
		raw, ok := pf.Palettes[string(b)]
		if !ok {
			return nil, fmt.Errorf("registry palettes: base %q missing", b)
		}
		pal, err := parsePalette(raw)
		if err != nil {
			return nil, fmt.Errorf("registry palette %q: %w", b, err)
		}
		r.palettes[b] = pal
	}
	return r, nil
}

func parsePalette(raw map[string]string) (map[string]theme.Color, error) {
	out := make(map[string]theme.Color, len(raw))
	for k, v := range raw {
		c, err := theme.ParseColor(v)
		if err != nil {
			return nil, fmt.Errorf("token %q: %w", k, err)
		}
		out[k] = c
	}
	return out, nil
}

// Tokens returns all tokens in registry order.
func (r *Registry) Tokens() []Token { return slices.Clone(r.tokens) }

// Lookup returns the metadata of key.
func (r *Registry) Lookup(key string) (Token, bool) {
	t, ok := r.byKey[key]
	return t, ok
}

// Count returns the number of tokens with the given status.
func (r *Registry) Count(s Status) int {
	n := 0
	for _, t := range r.tokens {
		if t.Status == s {
			n++
		}
	}
	return n
}

// BaseDefault returns the captured default of key in base, if known.
func (r *Registry) BaseDefault(b theme.Base, key string) (theme.Color, bool) {
	c, ok := r.palettes[b][key]
	return c, ok
}

// Resolve computes the effective color of every token the way Claude Code does:
// start from the base palette and apply an override only when the token exists
// in that palette (unknown tokens are ignored). Both the preview and the
// contrast checks use this single function, so they cannot disagree.
func (r *Registry) Resolve(t theme.Theme) map[string]theme.Color {
	base := r.palettes[t.EffectiveBase()]
	out := make(map[string]theme.Color, len(base))
	maps.Copy(out, base)
	for k, c := range t.Overrides {
		if _, ok := base[k]; ok {
			out[k] = c
		}
	}
	return out
}
