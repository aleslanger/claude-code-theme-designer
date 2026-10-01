package cli

import (
	"github.com/aleslanger/claude-code-theme-designer/internal/validate"
)

// printReport prints validation issues; info notes go to stdout as well so
// users see fullscreen-only and version caveats.
func (r *runner) printReport(rep validate.Report) {
	if len(rep.Issues) == 0 {
		return
	}
	r.printf("\n")
	for _, i := range rep.Issues {
		token := i.Token
		if token == "" {
			token = "-"
		}
		r.printf("  %-7s %-28s %s\n", i.Severity, token, i.Message)
	}
	r.printf("\n")
}
