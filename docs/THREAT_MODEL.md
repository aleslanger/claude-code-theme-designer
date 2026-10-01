# Threat model

## Assets

- User files in `~/.claude/` (Claude Code config, other themes) and elsewhere in `$HOME`.
- The user's terminal (escape sequences can retitle windows, rewrite the screen, or
  on some terminals trigger actions).
- Integrity of themes the user created by hand.

## Trust boundaries

| Input | Trust | Entry point |
|---|---|---|
| Imported / validated theme files | **untrusted** | `serialize.Decode` |
| Installed themes in `~/.claude/themes` | untrusted (other tools, plugins, users) | `serialize.Decode` |
| Designer config | semi-trusted (could be poisoned) | `store.loadConfig` + `Config.validate` |
| CLI arguments, keyboard input | user | `theme.ValidateSlug`, `theme.ParseColor` |
| Environment (`HOME`, `CLAUDE_CONFIG_DIR`, `TERM`) | user | `store.ResolvePaths`, `render.DetectProfile` |

## Threats and mitigations

| Threat | Mitigation | Test |
|---|---|---|
| Path traversal via theme name (`../../x`) | slug regex `[A-Za-z0-9_-]{1,64}`; `store.ThemeFile` proves `Dir(path) == dir` | `TestThemeFileRejectsTraversal`, `TestInstallAsRejectsTraversal` |
| Path traversal via import file name | slug derived from file name is validated; `--as` required otherwise | `TestImportNameFromFileMustBeSafe` |
| Symlink attack on destination (write through a planted link) | `checkTarget` refuses symlinks; writes use rename/link, which replace the entry instead of following it | `TestInstallRefusesSymlinkDestination` |
| Symlink / FIFO / device on read | reads of themes, drafts, config and backups use `O_NOFOLLOW`; all reads use `O_NONBLOCK` + regular-file check. Paths the user passes to `import`/`validate` may be symlinks (the user chose them); parent directories are followed on purpose (dotfile managers) | `TestReadFileLimited`, `TestReadFileLimitedDoesNotBlockOnFIFO` |
| TOCTOU on create | no-clobber publish via `link(2)` (atomic; fails if the target exists) | `TestWriteFileAtomicNoClobber` |
| TOCTOU on replace | rename is atomic; a concurrent writer can win, but no partial file and no followed symlink is possible | — |
| Partial writes / crash mid-write | temp file, then fsync, then rename, then dir fsync | `TestWriteFileAtomicReplacesAndLeavesNoTempFiles` |
| Overwriting user themes | never without confirmation or `--force`; backup first, stored outside the themes dir | `TestInstallCollision*`, `TestInstallOverwrite*` |
| Removing user themes | uninstall only for fingerprinted themes this tool wrote; edited ones need `--force`; backup kept | `TestUninstall*` |
| Malicious JSON: hooks / commands / extra keys | only `name`, `base`, `overrides`; unknown top-level keys rejected; values must be strings matching the color grammar; nothing is executed | `TestImportRejectsMaliciousInput` |
| Duplicate-key smuggling | duplicate keys rejected at every level | `TestDecodeRejectsMalformedInput` |
| Huge files / resource exhaustion | 256 KB limit (same as Claude Code), ≤512 overrides, ≤50 reported problems | `TestDecodeRejectsEncodingAttacks`, `TestImportRejectsHugeFileAndSpecialFiles` |
| Malformed Unicode / BOM | invalid UTF-8 and BOM rejected | `TestDecodeRejectsEncodingAttacks` |
| ANSI / OSC injection or display spoofing via `name`, keys, values, error messages | display names reject Unicode Cc/Cf/Co/Cs (controls, bidi marks and overrides, zero-width chars, BOM, tag chars); all printed data goes through `theme.Sanitize`; colors are re-encoded numerically, never echoed into SGR | `TestEncodeNeverEmitsUntrustedControlSequences`, `TestDecodeReportsProblemsWithoutEchoingControlCharacters` |
| Config poisoning (e.g. `current: "../../x"`) | config is validated; invalid → defaults, and the bad file is preserved aside; a config from a newer version is never overwritten | `TestCorruptOrPoisonedConfigFallsBackAndIsPreserved`, `TestNewerConfigIsNeverOverwritten` |
| Lost updates between concurrent runs (TUI + CLI) | every config change reloads from disk and writes under an `flock` | `TestConcurrentStoresDoNotLoseManagedEntries` |
| Hand-made file mistaken for a managed one | stale managed records are dropped when the file is gone; uninstall fingerprints and backs up the same bytes | `TestUninstallClearsStaleManagedEntry` |
| Command injection | the only subprocess is `claude --version` (fixed argv, no shell, timeout, output capped), and only in `doctor` | — |
| Writing theme data into other Claude Code configs | the tool never edits `settings.json` or anything outside `themes/` | — |

## Residual risks

- On non-Unix platforms reads check `Lstat` before `Open` (small race window; no `O_NOFOLLOW`), and the config lock is a no-op (reload-before-write still applies).
- Replacing an existing theme is atomic but not exclusive: a concurrent writer to `~/.claude/themes` can race the rename (no partial file, no symlink is followed).
- If an attacker already controls `~/.claude` or `$HOME`, they do not need this tool.
- Passthrough of unknown override keys means a file may carry colors for tokens
  this tool cannot preview. They remain constrained to valid color strings.
- Bubble Tea queries the terminal background (OSC 11) at startup. That query
  comes from the library, not from theme data.
