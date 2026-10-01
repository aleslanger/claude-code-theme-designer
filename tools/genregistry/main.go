// Command genregistry regenerates internal/registry/data/tokens.json from the
// official Claude Code documentation (the "Color token reference" in
// terminal-config.md) plus the captured base palettes.
//
// It is a development tool. The shipped binary never fetches anything.
//
//	go run ./tools/genregistry -docs terminal-config.md \
//	    -palettes internal/registry/data/palettes.json \
//	    -out internal/registry/data/tokens.json
//
// Fetch the docs first, e.g.:
//
//	curl -sL https://code.claude.com/docs/en/terminal-config.md -o terminal-config.md
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	defaultDocsURL    = "https://code.claude.com/docs/en/terminal-config#create-a-custom-theme"
	maxDocsBytes      = 4 << 20
	maxPalettesBytes  = 1 << 20
	registryFileMode  = 0o644
	registryDateStamp = "2006-01-02"
)

func main() {
	docsPath := flag.String("docs", "", "path to terminal-config.md (required)")
	palettesPath := flag.String("palettes", "internal/registry/data/palettes.json", "captured palettes")
	out := flag.String("out", "internal/registry/data/tokens.json", "output file")
	docsURL := flag.String("docs-url", defaultDocsURL, "URL recorded as the verified source")
	flag.Parse()
	if *docsPath == "" {
		fmt.Fprintln(os.Stderr, "genregistry: -docs is required")
		os.Exit(2)
	}
	if err := run(*docsPath, *palettesPath, *out, *docsURL); err != nil {
		fmt.Fprintln(os.Stderr, "genregistry:", err)
		os.Exit(1)
	}
}

func run(docsPath, palettesPath, out, docsURL string) error {
	docs, err := readLimited(docsPath, maxDocsBytes)
	if err != nil {
		return err
	}
	palRaw, err := readLimited(palettesPath, maxPalettesBytes)
	if err != nil {
		return err
	}
	var pal paletteFile
	if err := json.Unmarshal(palRaw, &pal); err != nil {
		return fmt.Errorf("parse palettes: %w", err)
	}
	documented, err := ParseDocs(string(docs))
	if err != nil {
		return err
	}
	reg, missing := Build(documented, pal, docsURL, time.Now().UTC().Format(registryDateStamp))
	for _, k := range missing {
		fmt.Fprintf(os.Stderr, "warning: documented token %q is not in the captured palettes (palettes may be outdated)\n", k)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), registryFileMode)
}

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() > limit {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, limit)
	}
	return os.ReadFile(path)
}
