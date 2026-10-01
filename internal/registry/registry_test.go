package registry

import (
	"strings"
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

func mustDefault(t *testing.T) *Registry {
	t.Helper()
	r, err := Default()
	if err != nil {
		t.Fatalf("embedded registry broken: %v", err)
	}
	return r
}

func TestEmbeddedRegistryLoads(t *testing.T) {
	r := mustDefault(t)
	if r.Count(Verified) == 0 || r.VerifiedAgainst == "" || r.DocsURL == "" {
		t.Fatalf("incomplete registry: %+v", r)
	}
}

func TestUserMessageTokensAreVerifiedWithSource(t *testing.T) {
	r := mustDefault(t)
	for _, k := range []string{"userMessageBackground", "userMessageBackgroundHover", "briefLabelYou", "text"} {
		tok, ok := r.Lookup(k)
		if !ok || tok.Status != Verified {
			t.Fatalf("%s must be a verified token, got %+v", k, tok)
		}
		if !strings.HasPrefix(tok.Source, "https://code.claude.com/docs/") {
			t.Fatalf("%s has no documentation source: %q", k, tok.Source)
		}
	}
	if tok, _ := r.Lookup("userMessageBackgroundHover"); !tok.FullscreenOnly {
		t.Fatal("userMessageBackgroundHover is documented as fullscreen-only")
	}
}

func TestNoInventedUserMessageTextToken(t *testing.T) {
	r := mustDefault(t)
	for _, k := range []string{"userMessageText", "userMessageForeground", "userText"} {
		if _, ok := r.Lookup(k); ok {
			t.Fatalf("registry must not contain undocumented token %q", k)
		}
	}
}

func TestEveryTokenHasCompleteMetadata(t *testing.T) {
	r := mustDefault(t)
	for _, tok := range r.Tokens() {
		if tok.Key == "" || tok.DisplayName == "" || tok.Category == "" || tok.Description == "" || tok.Source == "" {
			t.Errorf("incomplete metadata: %+v", tok)
		}
		if tok.Role != RoleForeground && tok.Role != RoleBackground {
			t.Errorf("%s: bad role %q", tok.Key, tok.Role)
		}
	}
}

func TestEveryBaseHasADefaultForEveryToken(t *testing.T) {
	r := mustDefault(t)
	for _, b := range theme.Bases {
		for _, tok := range r.Tokens() {
			if _, ok := r.BaseDefault(b, tok.Key); !ok {
				t.Errorf("base %s lacks default for %s", b, tok.Key)
			}
		}
	}
}

func TestSectionsListOnlyVerifiedTokensUserMessagesFirst(t *testing.T) {
	r := mustDefault(t)
	secs := r.Sections()
	if secs[0].Title != "User messages" || secs[0].Keys[0] != "userMessageBackground" {
		t.Fatalf("user messages must come first, got %+v", secs[0])
	}
	if len(secs[0].Unsupported) == 0 {
		t.Fatal("unsupported capabilities must be listed explicitly")
	}
	seen := map[string]bool{}
	for _, s := range secs {
		for _, k := range s.Keys {
			tok, _ := r.Lookup(k)
			if tok.Status != Verified {
				t.Errorf("section %q offers unverified token %s", s.Title, k)
			}
			if seen[k] {
				t.Errorf("token %s listed twice", k)
			}
			seen[k] = true
		}
	}
	if len(seen) != r.Count(Verified) {
		t.Fatalf("sections cover %d tokens, registry has %d verified", len(seen), r.Count(Verified))
	}
}

func TestResolveIgnoresUnknownTokensLikeClaudeCode(t *testing.T) {
	r := mustDefault(t)
	th := theme.Theme{Overrides: map[string]theme.Color{
		"text":          theme.MustParseColor("#123456"),
		"notARealToken": theme.MustParseColor("#654321"),
	}}
	res := r.Resolve(th)
	if res["text"].String() != "#123456" {
		t.Fatal("override not applied")
	}
	if _, ok := res["notARealToken"]; ok {
		t.Fatal("unknown token must not be resolved")
	}
	def, _ := r.BaseDefault(theme.BaseDark, "claude")
	if !res["claude"].Equal(def) {
		t.Fatal("non-overridden token must inherit from base")
	}
}

func TestLoadRejectsBrokenData(t *testing.T) {
	if _, err := Load([]byte(`{"tokens":[{"key":"a","status":"maybe"}]}`), palettesJSON); err == nil {
		t.Fatal("unknown status must fail")
	}
	if _, err := Load(tokensJSON, []byte(`{"palettes":{}}`)); err == nil {
		t.Fatal("missing palettes must fail")
	}
}
