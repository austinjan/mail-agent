# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Documentation layout

All plans, implementation details, and progress tracking live under `/docs`. Create the directory if it does not exist.

- `/docs/plans` — plans (design, approach, proposals)
- `/docs/tasks` — implementation items and progress status
- `/docs/references` — reference material

## Status

The MVP read-mail path is implemented through Task 11:

- Go module and CLI entrypoint under `cmd/mail-agent`.
- Shared mail/config types under `internal/mail` and `internal/config`.
- SQLite store with deduplication and content-hashed attachment storage under `internal/store`.
- IMAP source, since parser, and MIME parsing under `internal/source`.
- Core fetch -> dedup -> persist pipeline with structured `slog` events under `internal/core`.
- Progress tracking under `docs/tasks/2026-04-22-mvp-read-mail`.

Task 12 remains pending because it requires live Gmail/IMAP credentials and acceptance smoke testing against a real mailbox. Do not mark Task 12 complete until those acceptance checks are recorded.
