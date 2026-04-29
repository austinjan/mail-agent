# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation layout

All plans, implementation details, and progress tracking live under `/docs`. Create the directory if it does not exist.

- `/docs/plans` — plans (design, approach, proposals)
- `/docs/tasks` — implementation items and progress status
- `/docs/references` — reference material

## Stack

- Language: Go 1.22+
- Module: `github.com/austinjan/mail-agent`
- IMAP client: `github.com/emersion/go-imap/v2`
- SQLite driver: `modernc.org/sqlite` (pure Go, no CGO)
- YAML: `gopkg.in/yaml.v3`
- Logging: stdlib `log/slog`
- Testing: stdlib `testing`

## Project structure

```text
cmd/mail-agent/        # CLI entrypoint (Task 11, pending)
internal/
  config/              # Config loading (Task 03, completed)
  mail/                # Shared data types (Task 02, completed)
  source/              # IMAP fetch + MIME parsing (Tasks 07-09, in progress on PR)
  store/               # SQLite persistence (Tasks 04-06, completed)
  core/                # Pipeline (Task 10, pending)
docs/
  plans/               # Design documents
  tasks/               # Task breakdown and progress
```

## Status

MVP Read Mail is in progress. Tasks 01-06 are completed. Tasks 07-09 are under review in an open PR. Tasks 10-12 are still pending.

See `docs/tasks/2026-04-22-mvp-read-mail/README.md` for the detailed task checklist.
