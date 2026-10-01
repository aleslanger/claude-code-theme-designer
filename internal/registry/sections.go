package registry

// Section groups tokens for the editor. Only verified tokens are listed;
// unverified tokens are preserved in files but never offered for editing.
type Section struct {
	Title       string
	Keys        []string
	Unsupported []Unsupported
	// Labels optionally replace generated display names with task-oriented ones.
	Labels map[string]string
}

// Label returns the editor label for key in this section.
func (s Section) Label(key string, reg *Registry) string {
	if l, ok := s.Labels[key]; ok {
		return l
	}
	if t, ok := reg.Lookup(key); ok {
		return t.DisplayName
	}
	return key
}

// Unsupported documents a requested capability that the theme API lacks, so
// the UI can say so instead of inventing a token.
type Unsupported struct {
	Label  string
	Reason string
}

// userMessageSection is the curated first section: everything that helps tell
// your prompts apart from Claude's replies.
var userMessageSection = Section{
	Title: "User messages",
	Keys: []string{
		"userMessageBackground",
		"userMessageBackgroundHover",
		"briefLabelYou",
		"briefLabelClaude",
		"bashMessageBackgroundColor",
		"memoryBackgroundColor",
		"selectionBg",
	},
	Labels: map[string]string{
		"userMessageBackground":      "Background",
		"userMessageBackgroundHover": "Hover / expanded background",
		"briefLabelYou":              `Label "You"`,
		"briefLabelClaude":           `Label "Claude"`,
		"bashMessageBackgroundColor": "! shell entry background",
		"memoryBackgroundColor":      "# memory entry background",
		"selectionBg":                "Mouse selection background",
	},
	Unsupported: []Unsupported{
		{
			Label:  "User message text color",
			Reason: "no dedicated token; only the global `text` token exists (affects all text)",
		},
		{
			Label:  "Border / padding / font",
			Reason: "themes only control colors",
		},
	},
}

// Sections returns editor sections: the curated user-message section first,
// then every remaining verified token grouped by its documented category.
func (r *Registry) Sections() []Section {
	used := map[string]bool{}
	first := Section{Title: userMessageSection.Title, Unsupported: userMessageSection.Unsupported, Labels: userMessageSection.Labels}
	for _, k := range userMessageSection.Keys {
		if t, ok := r.byKey[k]; ok && t.Status == Verified {
			first.Keys = append(first.Keys, k)
			used[k] = true
		}
	}
	out := []Section{first}
	index := map[string]int{}
	for _, t := range r.tokens {
		if t.Status != Verified || used[t.Key] {
			continue
		}
		i, ok := index[t.Category]
		if !ok {
			i = len(out)
			index[t.Category] = i
			out = append(out, Section{Title: t.Category})
		}
		out[i].Keys = append(out[i].Keys, t.Key)
	}
	return out
}
