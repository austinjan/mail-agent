# mail-agent

A small agent that reads mail from an IMAP mailbox within a caller-specified
time range, persists per-mail state so the same mail is never processed twice,
and is structured so the trigger, mail source, and storage backend can be
swapped later without rewriting core logic.

Tracking issues: [#1 讀取信件](https://github.com/austinjan/mail-agent/issues/1),
[#2 Dispatch task based on mail](https://github.com/austinjan/mail-agent/issues/2) (out of MVP scope).

## Status

MVP read-mail implementation and live Gmail/IMAP smoke-test acceptance are
complete through Task 12. The full design and task breakdown live under
[`docs/`](./docs):

- [`docs/plans/2026-04-22-mvp-read-mail-design.md`](./docs/plans/2026-04-22-mvp-read-mail-design.md) — design, decisions, schema, architecture
- [`docs/tasks/2026-04-22-mvp-read-mail/`](./docs/tasks/2026-04-22-mvp-read-mail/) — implementation task breakdown

The implementation includes config loading, mail data types, SQLite-backed
deduplication and attachment storage, IMAP fetching with MIME parsing, the core
fetch/dedup/persist pipeline, the `mail-agent` CLI entrypoint, and an OCR-free
extraction pipeline for stored mail bodies and attachments.

## Usage

```bash
mail-agent read --since=3d                   # primary command
mail-agent read --since=3d --folder=INBOX    # override folder from config
mail-agent extract enqueue --since=24h       # create extraction jobs
mail-agent extract run --limit=20            # process pending extraction jobs
mail-agent extract show --mail-id=123        # review extracted fields
mail-agent extract export --out=fields.csv   # export extracted fields to CSV
mail-agent version
```

`--since` accepts durations (`3d`, `1w`, `24h`) or an RFC-3339 timestamp. If it
is omitted, the CLI uses `defaults.since` from `config.yaml`. A system cron is
the intended trigger for MVP.

## Stack

- Go 1.22+
- IMAP: [`github.com/emersion/go-imap/v2`](https://github.com/emersion/go-imap)
- SQLite: [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- YAML: [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3)
- Logging: standard library `log/slog` JSON handler

## Configuration

A single `config.yaml` holds IMAP credentials and paths. `config.yaml` is
git-ignored; [`config.example.yaml`](./config.example.yaml) ships as the
template. Gmail requires an App Password (2FA -> app passwords), not the account
login password. See the design doc for the full schema.

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
