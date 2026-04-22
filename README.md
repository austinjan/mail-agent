# mail-agent

A small agent that reads mail from an IMAP mailbox within a caller-specified
time range, persists per-mail state so the same mail is never processed twice,
and is structured so the trigger, mail source, and storage backend can be
swapped later without rewriting core logic.

Tracking issues: [#1 讀取信件](https://github.com/austinjan/mail-agent/issues/1),
[#2 Dispatch task based on mail](https://github.com/austinjan/mail-agent/issues/2) (out of MVP scope).

## Status

Design complete, implementation not started. The full design and task
breakdown live under [`docs/`](./docs):

- [`docs/plans/2026-04-22-mvp-read-mail-design.md`](./docs/plans/2026-04-22-mvp-read-mail-design.md) — design, decisions, schema, architecture
- [`docs/tasks/2026-04-22-mvp-read-mail/`](./docs/tasks/2026-04-22-mvp-read-mail/) — implementation task breakdown

## Planned usage

```bash
mail-agent read --since=3d                   # primary command
mail-agent read --since=3d --folder=INBOX    # override folder from config
mail-agent version
```

`--since` accepts durations (`3d`, `1w`, `24h`) or an RFC-3339 timestamp and is
mandatory on every invocation. A system cron is the intended trigger for MVP.

## Planned stack

- Go 1.22+
- IMAP: [`github.com/emersion/go-imap/v2`](https://github.com/emersion/go-imap)
- SQLite: [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)

## Configuration

A single `config.yaml` holds IMAP credentials and paths. `config.yaml` is
git-ignored; a `config.example.yaml` template will ship in git. Gmail requires
an App Password (2FA → app passwords), not the account login password. See the
design doc for the full schema.

## Layout

```
cmd/mail-agent/        CLI entrypoint
internal/core/         fetch → dedup → persist pipeline
internal/source/       Source interface + IMAP implementation
internal/store/        Store interface + SQLite implementation
internal/mail/         shared Mail / Attachment types
internal/config/       YAML config parsing
docs/                  plans, tasks, references
```

## License

TBD.
