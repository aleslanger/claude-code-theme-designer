package main

import (
	"maps"
	"slices"
	"strings"
)

type paletteFile struct {
	CapturedFrom string                       `json:"capturedFrom"`
	Palettes     map[string]map[string]string `json:"palettes"`
}

type registryFile struct {
	RegistryVersion string      `json:"registryVersion"`
	DocsURL         string      `json:"docsUrl"`
	VerifiedAgainst string      `json:"verifiedAgainst"`
	Tokens          []tokenJSON `json:"tokens"`
}

type tokenJSON struct {
	Key            string `json:"key"`
	DisplayName    string `json:"displayName"`
	Category       string `json:"category"`
	Role           string `json:"role"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Source         string `json:"source"`
	SinceVersion   string `json:"sinceVersion,omitempty"`
	FullscreenOnly bool   `json:"fullscreenOnly,omitempty"`
}

const (
	statusVerified       = "verified"
	statusUnverified     = "unverified"
	undocumentedCategory = "Undocumented"
	referenceBasePalette = "dark"
)

// Build merges documented tokens with the captured palettes. Documented tokens
// are "verified"; palette keys absent from the docs are "unverified". It also
// returns documented keys missing from the palettes.
func Build(docs []DocToken, pal paletteFile, docsURL, date string) (registryFile, []string) {
	known := pal.Palettes[referenceBasePalette]
	reg := registryFile{RegistryVersion: date, DocsURL: docsURL, VerifiedAgainst: pal.CapturedFrom}
	seen := map[string]bool{}
	var missing []string
	for _, d := range docs {
		if seen[d.Key] {
			continue
		}
		seen[d.Key] = true
		if _, ok := known[d.Key]; !ok {
			missing = append(missing, d.Key)
		}
		reg.Tokens = append(reg.Tokens, tokenJSON{
			Key:            d.Key,
			DisplayName:    DisplayName(d.Key),
			Category:       d.Category,
			Role:           role(d.Key, d.Description),
			Description:    d.Description,
			Status:         statusVerified,
			Source:         docsURL + " (Color token reference › " + d.Category + ")",
			SinceVersion:   d.SinceVersion,
			FullscreenOnly: d.FullscreenOnly,
		})
	}
	for _, k := range slices.Sorted(maps.Keys(known)) {
		if seen[k] {
			continue
		}
		reg.Tokens = append(reg.Tokens, tokenJSON{
			Key:         k,
			DisplayName: DisplayName(k),
			Category:    undocumentedCategory,
			Role:        role(k, ""),
			Description: "Present in the built-in palette but not in the official docs; may change without notice",
			Status:      statusUnverified,
			Source:      pal.CapturedFrom + " built-in palette (undocumented)",
		})
	}
	return reg, missing
}

func role(key, desc string) string {
	d := strings.ToLower(desc)
	k := strings.ToLower(key)
	if strings.Contains(d, "background") || strings.Contains(d, "highlight") ||
		strings.HasSuffix(k, "bg") || strings.Contains(k, "background") {
		return "background"
	}
	return "foreground"
}
