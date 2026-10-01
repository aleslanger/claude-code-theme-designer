package presets

import "github.com/aleslanger/claude-code-theme-designer/internal/theme"

// inspired presets approximate public palettes. They are not the official
// schemes; colors were adjusted where needed to meet the contrast checks.
func inspired() []Preset {
	return []Preset{
		darkPalette("nord-like", "Nord-like", "Cool blue-grey, inspired by Nord", palette{
			userBG: "#3b4252", hover: "#434c5e", you: "#88c0d0", them: "#ebcb8b", text: "#eceff4",
			accent: "#88c0d0", success: "#a3be8c", error: "#d57780", warning: "#ebcb8b", suggestion: "#81a1c1",
		}),
		darkPalette("dracula-like", "Dracula-like", "Purple and pink accents, inspired by Dracula", palette{
			userBG: "#44475a", hover: "#4f5269", you: "#d4b5fc", them: "#ff79c6", text: "#f8f8f2",
			accent: "#bd93f9", success: "#50fa7b", error: "#ff5555", warning: "#f1fa8c", suggestion: "#8be9fd",
		}),
		darkPalette("gruvbox-like", "Gruvbox-like", "Warm retro, inspired by Gruvbox", palette{
			userBG: "#3c3836", hover: "#504945", you: "#fabd2f", them: "#83a598", text: "#ebdbb2",
			accent: "#fe8019", success: "#b8bb26", error: "#fb4934", warning: "#fabd2f", suggestion: "#83a598",
		}),
		darkPalette("solarized-like", "Solarized-like", "Low-glare teal, inspired by Solarized dark", palette{
			userBG: "#073642", hover: "#0b4552", you: "#3fbfb3", them: "#6c9ee0", text: "#eee8d5",
			accent: "#e3743f", success: "#a5b82c", error: "#e6524f", warning: "#c9a227", suggestion: "#6c9ee0",
		}),
		darkPalette("tokyo-night-like", "Tokyo Night-like", "Indigo night with neon blue, inspired by Tokyo Night", palette{
			userBG: "#2f3549", hover: "#3b4261", you: "#7aa2f7", them: "#bb9af7", text: "#c0caf5",
			accent: "#7aa2f7", success: "#9ece6a", error: "#f7768e", warning: "#e0af68", suggestion: "#7dcfff",
		}),
		darkPalette("catppuccin-mocha-like", "Catppuccin Mocha-like", "Soft pastels on dark, inspired by Catppuccin Mocha", palette{
			userBG: "#313244", hover: "#45475a", you: "#cba6f7", them: "#fab387", text: "#cdd6f4",
			accent: "#cba6f7", success: "#a6e3a1", error: "#f38ba8", warning: "#f9e2af", suggestion: "#89b4fa",
		}),
		darkPalette("one-dark-like", "One Dark-like", "Balanced blue and magenta, inspired by One Dark", palette{
			userBG: "#2c313c", hover: "#3a404b", you: "#61afef", them: "#c678dd", text: "#c8ccd4",
			accent: "#c678dd", success: "#98c379", error: "#e06c75", warning: "#e5c07b", suggestion: "#56b6c2",
		}),
		darkPalette("monokai-like", "Monokai-like", "Punchy lime and orange, inspired by Monokai", palette{
			userBG: "#3e3d32", hover: "#49483e", you: "#a6e22e", them: "#fd971f", text: "#f8f8f2",
			accent: "#f92672", success: "#a6e22e", error: "#ff5c8a", warning: "#e6db74", suggestion: "#66d9ef",
		}),
		darkPalette("rose-pine-like", "Rosé Pine-like", "Muted rose and iris, inspired by Rosé Pine", palette{
			userBG: "#26233a", hover: "#393552", you: "#c4a7e7", them: "#ebbcba", text: "#e0def4",
			accent: "#ebbcba", success: "#9ccfd8", error: "#eb6f92", warning: "#f6c177", suggestion: "#9ccfd8",
		}),
		darkPalette("everforest-like", "Everforest-like", "Calm green-grey, inspired by Everforest", palette{
			userBG: "#343f44", hover: "#3d484d", you: "#a7c080", them: "#dbbc7f", text: "#d3c6aa",
			accent: "#e69875", success: "#a7c080", error: "#e67e80", warning: "#dbbc7f", suggestion: "#7fbbb3",
		}),
		darkPalette("kanagawa-like", "Kanagawa-like", "Ink blue and autumn orange, inspired by Kanagawa", palette{
			userBG: "#2a2a37", hover: "#363646", you: "#7e9cd8", them: "#ffa066", text: "#dcd7ba",
			accent: "#957fb8", success: "#98bb6c", error: "#e46876", warning: "#e6c384", suggestion: "#7fb4ca",
		}),
		lightPalette("catppuccin-latte-like", "Catppuccin Latte-like", "Soft pastels on light, inspired by Catppuccin Latte", palette{
			userBG: "#e6e9ef", hover: "#eff1f5", you: "#7130d9", them: "#b8460a", text: "#3c3f58",
			accent: "#8839ef", success: "#2f7a1f", error: "#c20d35", warning: "#8f5809", suggestion: "#1e66f5",
		}),
		lightPalette("github-light-like", "GitHub Light-like", "Clean blue on white, inspired by GitHub Light", palette{
			userBG: "#ddf4ff", hover: "#eef8ff", you: "#0969da", them: "#8250df", text: "#1f2328",
			accent: "#8250df", success: "#1a7f37", error: "#cf222e", warning: "#9a6700", suggestion: "#0969da",
		}),
	}
}

// palette is the shared shape of the inspired presets.
type palette struct {
	userBG, hover, you, them, text              string
	accent, success, error, warning, suggestion string
}

func (p palette) overrides() map[string]string {
	return map[string]string{
		"userMessageBackground":      p.userBG,
		"userMessageBackgroundHover": p.hover,
		"briefLabelYou":              p.you,
		"briefLabelClaude":           p.them,
		"text":                       p.text,
		"claude":                     p.accent,
		"success":                    p.success,
		"error":                      p.error,
		"warning":                    p.warning,
		"suggestion":                 p.suggestion,
	}
}

func darkPalette(slug, name, desc string, p palette) Preset {
	return Preset{Slug: slug, Category: CategoryInspired, Description: desc + " (unofficial)",
		Theme: build(name, theme.BaseDark, p.overrides())}
}

func lightPalette(slug, name, desc string, p palette) Preset {
	return Preset{Slug: slug, Category: CategoryInspired, Description: desc + " (unofficial)",
		Theme: build(name, theme.BaseLight, p.overrides())}
}
