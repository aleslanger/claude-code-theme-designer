# Security policy

## Supported versions

Only the latest release receives fixes.

## Reporting a vulnerability

Please **do not open a public issue**. Report privately via
[GitHub security advisories](https://github.com/aleslanger/claude-code-theme-designer/security/advisories/new).

Include the claude-theme version (`claude-theme version`), your OS, and steps
or a theme file that reproduces the problem. You should get a response within
a week.

## Scope

In scope: anything that lets a theme file, config file or name make the tool
write outside its directories, overwrite or delete files it should not touch,
execute code, or emit uncontrolled terminal escape sequences. The design and
known residual risks are documented in [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md).

Out of scope: Claude Code itself (report to Anthropic), and attacks that
require an attacker who already controls your home directory.
