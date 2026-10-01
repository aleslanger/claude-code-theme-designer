package presets

import "github.com/aleslanger/claude-code-theme-designer/internal/theme"

func accessible() []Preset {
	return []Preset{
		{
			Slug: "high-contrast-dark", Category: CategoryAccessibility,
			Description: "Dark; maximum legibility, white text on navy prompts",
			Theme: build("High Contrast Dark", theme.BaseDark, map[string]string{
				"userMessageBackground":      "#0b2a4a",
				"userMessageBackgroundHover": "#123a63",
				"briefLabelYou":              "#ffd700",
				"briefLabelClaude":           "#00e5ff",
				"text":                       "#ffffff",
				"success":                    "#5cff5c",
				"error":                      "#ff7b7b",
				"warning":                    "#ffd700",
				"suggestion":                 "#00e5ff",
				"promptBorder":               "#ffffff",
			}),
		},
		{
			Slug: "high-contrast-light", Category: CategoryAccessibility,
			Description: "Light; maximum legibility, black text on yellow prompts",
			Theme: build("High Contrast Light", theme.BaseLight, map[string]string{
				"userMessageBackground":      "#fff3b0",
				"userMessageBackgroundHover": "#ffe680",
				"briefLabelYou":              "#0000b3",
				"briefLabelClaude":           "#8b0000",
				"text":                       "#000000",
				"success":                    "#006400",
				"error":                      "#b00000",
				"warning":                    "#7a4a00",
				"suggestion":                 "#0000b3",
				"promptBorder":               "#000000",
			}),
		},
		{
			Slug: "colorblind-safe", Category: CategoryAccessibility,
			Description: "Dark-daltonized base with Okabe-Ito orange/blue labels",
			Theme: build("Colorblind Safe", theme.BaseDarkDaltonized, map[string]string{
				"userMessageBackground":      "#2b3a55",
				"userMessageBackgroundHover": "#34476a",
				"briefLabelYou":              "#e69f00",
				"briefLabelClaude":           "#56b4e9",
			}),
		},
		{
			Slug: "ansi-16", Category: CategoryAccessibility,
			Description: "Dark-ansi base, 16 terminal colors only; follows your terminal palette",
			Theme: build("ANSI 16", theme.BaseDarkANSI, map[string]string{
				"userMessageBackground": "ansi:blue",
				"briefLabelYou":         "ansi:yellowBright",
				"briefLabelClaude":      "ansi:cyanBright",
			}),
		},
	}
}
