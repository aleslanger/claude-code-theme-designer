package cli

import "runtime/debug"

const (
	devVersion       = "dev"
	develModuleBuild = "(devel)" // reported for builds from a local checkout
)

// currentVersion prefers the version injected with -ldflags (release
// archives, make) and falls back to the module version Go records for
// `go install …@vX.Y.Z` builds.
func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	return resolveVersion(Version, info, ok)
}

func resolveVersion(injected string, info *debug.BuildInfo, ok bool) string {
	if injected != devVersion || !ok || info == nil {
		return injected
	}
	if v := info.Main.Version; v != "" && v != develModuleBuild {
		return v
	}
	return injected
}
