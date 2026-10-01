package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	withModule := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	cases := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		ok       bool
		want     string
	}{
		{"ldflags win", "v1.2.3", withModule("v9.9.9"), true, "v1.2.3"},
		{"go install", devVersion, withModule("v0.1.1"), true, "v0.1.1"},
		{"local checkout", devVersion, withModule(develModuleBuild), true, devVersion},
		{"no build info", devVersion, nil, false, devVersion},
	}
	for _, tc := range cases {
		if got := resolveVersion(tc.injected, tc.info, tc.ok); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
