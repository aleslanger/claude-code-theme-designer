package presets

import "github.com/aleslanger/claude-code-theme-designer/internal/theme"

func originals() []Preset {
	return []Preset{
		{
			Slug: "prompt-contrast", Category: CategoryOriginal,
			Description: "Dark; violet background and label make your prompts stand out",
			Theme: build("Prompt Contrast", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#332b4f",
				"userMessageBackgroundHover": "#403660",
				"briefLabelYou":              "#c4a7ff",
			}),
		},
		{
			Slug: "prompt-contrast-light", Category: CategoryOriginal,
			Description: "Light; lavender background and deep violet label",
			Theme: build("Prompt Contrast Light", theme.BaseLight, map[string]string{
				"userMessageBackground":      "#ece6fb",
				"userMessageBackgroundHover": "#f4f0fd",
				"briefLabelYou":              "#5b3cc4",
			}),
		},
		{
			Slug: "ocean", Category: CategoryOriginal,
			Description: "Dark; deep teal prompts with cyan and amber labels",
			Theme: build("Ocean", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#0f3a4a",
				"userMessageBackgroundHover": "#164b5e",
				"briefLabelYou":              "#5fd7ff",
				"briefLabelClaude":           "#ffaf5f",
				"claude":                     "#ffaf5f",
				"suggestion":                 "#5fd7ff",
				"promptBorder":               "#3a7a8c",
			}),
		},
		{
			Slug: "sunset", Category: CategoryOriginal,
			Description: "Dark; warm plum prompts with peach and coral labels",
			Theme: build("Sunset", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#4a2a3a",
				"userMessageBackgroundHover": "#5a3346",
				"briefLabelYou":              "#ffb86b",
				"briefLabelClaude":           "#ff8fa3",
				"claude":                     "#ff8fa3",
				"suggestion":                 "#ffb86b",
				"promptBorder":               "#8a5a6a",
			}),
		},
		{
			Slug: "forest", Category: CategoryOriginal,
			Description: "Dark; moss-green prompts with lime and sand labels",
			Theme: build("Forest", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#263826",
				"userMessageBackgroundHover": "#304630",
				"briefLabelYou":              "#b5e48c",
				"briefLabelClaude":           "#e9c46a",
				"claude":                     "#e9c46a",
				"suggestion":                 "#b5e48c",
				"promptBorder":               "#5a7a5a",
			}),
		},
		{
			Slug: "minimal-dark", Category: CategoryOriginal,
			Description: "Dark; subtle grey prompt background and a single blue accent",
			Theme: build("Minimal Dark", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#2a2a2a",
				"userMessageBackgroundHover": "#353535",
				"briefLabelYou":              "#9ecbff",
			}),
		},
		{
			Slug: "minimal-light", Category: CategoryOriginal,
			Description: "Light; pale blue prompt background",
			Theme: build("Minimal Light", theme.BaseLight, map[string]string{
				"userMessageBackground":      "#e3ebf7",
				"userMessageBackgroundHover": "#eef3fa",
				"briefLabelYou":              "#1f4fbf",
			}),
		},
	}
}
