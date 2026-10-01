//go:build !unix

package doctor

func writability(string) string { return "writability not checked on this platform" }
