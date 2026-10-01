# Contributing

## Hard rules

1. **No patching of Claude Code**: no binary/`cli.js` edits, no injection, no API
   interception, no reading of conversations. Only theme files the docs describe.
2. **No invented tokens.** A token becomes editable only when it appears in the
   official docs. Regenerate the registry with `tools/genregistry` and review
   the diff; do not hand-edit `tokens.json`.
3. Every behavior change comes with a test. Bug fixes come with a regression test.

## Workflow

```sh
go test -race ./...
go vet ./...
gofmt -l .
```

Snapshot tests (`internal/render/testdata`, `internal/tui/testdata`) fail on any
visual change. If the change is intended, run `go test ./internal/render ./internal/tui -update`
and include the reviewed golden files in the commit.

Regenerate the README screenshot after visible editor changes:
`go run ./tools/screenshot -o docs/screenshot.svg` and
`go run ./tools/screenshot -gallery -o docs/presets.svg` (after preset changes).

Commit messages: `<type>: <description>` (`feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`).

## Presets

New presets must use only verified tokens and pass
`TestPresetsAreValidVerifiedAndReadable` (no contrast warnings). Name palettes
inspired by existing schemes "<Name>-like" and do not present them as official.

## Security

Report vulnerabilities privately via GitHub security advisories, not public issues.
Read [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md) before touching `serialize`, `store` or `render`.
