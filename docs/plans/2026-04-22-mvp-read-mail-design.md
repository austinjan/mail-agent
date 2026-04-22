# MVP: Read Mail — Design

- Date: 2026-04-22
- Status: Brainstorm complete — ready for implementation plan
- Target issue: [#1 讀取信件](https://github.com/austinjan/mail-agent/issues/1)
- Related: [#2 Dispatch task based on mail](https://github.com/austinjan/mail-agent/issues/2) (out of MVP scope, but design must not block it)

## Goal

Build the first capability of the mail-agent: **read mail from an IMAP mailbox** within a user-specified time range, persist per-mail state so the same mail is never processed twice, and do so through a layered architecture that allows the trigger mechanism, mail source, and storage backend to be swapped later without rewriting core logic.

## Scope

**In scope (MVP)**:
- Connect to a single IMAP mailbox (Gmail via IMAP on day one; provider-agnostic by design).
- Fetch mails within a caller-specified time range (`--since=3d`, `--since=1w`, etc.).
- Fetch every mail completely — all headers, plain-text body, HTML body, attachments.
- Deduplicate against previously-seen mails.
- Persist mail metadata + bodies to local SQLite; store attachment files separately on disk.
- Log structured events (JSON) for every step for later analysis.

**Out of scope (MVP)**:
- Dispatching tasks based on mail content (Issue #2).
- Replying, labelling, moving, or deleting mail.
- Backfilling historical mail on first run.
- Multi-account / multi-mailbox.
- IMAP IDLE / real-time push.

## Decisions

### D1. Invocation pattern — cron-triggered CLI

Run as a one-shot CLI (`mail-agent read --since=...`). A system cron schedules it (e.g. every 5 minutes).

**Why**: easiest to write, easiest to debug, each run is a pure function. If the process crashes, cron restarts it next tick — no supervisor required.

**Flexibility clause** (user-requested): the trigger mechanism is explicitly decoupled from core logic. Replacing cron with a long-running daemon or IMAP IDLE push later must not require changes to the fetch / store / dedup code.

```
Trigger (swappable)               Core logic (stable)
┌──────────────┐                 ┌────────────────────┐
│ Cron (MVP)   │                 │ fetch_mails(since) │
│ Daemon       │ ───invoke────►  │ dedup + persist    │
│ IDLE         │                 │ log summary        │
└──────────────┘                 └────────────────────┘
```

### D2. Language and runtime — Go

Go 1.22+. IMAP client: `github.com/emersion/go-imap/v2`. SQLite driver: `modernc.org/sqlite` (pure-Go, no CGO).

**Why Go**: single binary deployment, strong typing clarifies interface boundaries (serves the flexibility goal), goroutines make the future daemon / IDLE transitions natural.

### D3. Mail source — generic IMAP

Protocol: IMAP over TLS (port 993). No vendor-specific API.

**Why**: works against any mail provider; Gmail requires an App Password (2FA-gated), which the config path accommodates.

### D4. Time range is mandatory on every invocation

Every read operation requires an explicit `--since` (examples: `3d`, `1w`, `24h`, or an absolute RFC-3339 timestamp). There is no implicit "read everything new" mode.

**Why**: explicit scope prevents runaway fetches and ambiguous behaviour. Re-running the same command is safe because of D6 (dedup).

### D5. Ignore history on first run

The first execution does **not** back-fill existing mail. The cutoff is the caller's `--since` value; anything older is invisible to the agent regardless of what's in the mailbox.

**Why**: this is an agent, not a backup tool. Historical mail predates the agent's responsibility and should not trigger future tasks (Issue #2).

### D6. Deduplication

**Primary key**: `(UIDVALIDITY, UID, Folder)` stored in SQLite. Every fetched mail is checked against this key; hits are skipped.

**Fallback**: RFC 5322 `Message-ID` header is stored as an additional column. Used as a backup check when UIDVALIDITY changes (rare — happens on server rebuilds).

**UIDVALIDITY change handling**: detected by comparing the current value against the last-known. On change, the agent re-establishes its baseline at the current point in time (consistent with D5 — does not back-fill).

**User-facing guarantee**: the same mail is never processed twice. Internal strategy is an implementation detail.

### D7. Output and persistence — stdout log + SQLite + attachments on disk

Structured JSON events go to stdout. Mail metadata and bodies go to SQLite. Attachment file contents go to a separate directory.

**Storage behind an interface** so MVP's SQLite can be replaced for production (Postgres, cloud DB, object store, etc.) without touching core logic:

```go
type Store interface {
    SaveMail(m Mail) (mailID int64, err error)
    HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error)
    HasSeenByMessageID(messageID string) (bool, error)
    SaveAttachment(mailID int64, a Attachment) error
}
```

### D8. Fields and attachments — fetch everything, no size limits

Every mail is fetched in full. No body truncation, no attachment size cap. Disk is cheap; storage fidelity matters more than space.

**SQLite schema**:

```sql
CREATE TABLE mails (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    uid_validity  INTEGER NOT NULL,
    uid           INTEGER NOT NULL,
    folder        TEXT NOT NULL,
    message_id    TEXT,
    subject       TEXT,
    from_addr     TEXT,
    to_addrs      TEXT,         -- JSON array
    cc_addrs      TEXT,         -- JSON array
    reply_to      TEXT,
    in_reply_to   TEXT,
    refs          TEXT,         -- JSON array (conversation threading)
    flags         TEXT,         -- JSON array (\Seen, \Flagged, Gmail labels, ...)
    received_at   TIMESTAMP,
    body_text     TEXT,         -- plain-text body, full
    body_html     TEXT,         -- HTML body, full
    raw_headers   TEXT,         -- original headers blob
    fetched_at    TIMESTAMP NOT NULL,
    UNIQUE (uid_validity, uid, folder)
);

CREATE TABLE attachments (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    mail_id       INTEGER NOT NULL REFERENCES mails(id),
    filename      TEXT,
    content_type  TEXT,
    size_bytes    INTEGER,
    sha256        TEXT NOT NULL,  -- content fingerprint
    file_path     TEXT NOT NULL   -- relative path on disk
);
```

**Attachment file layout — content-hashed**:

```
./attachments/
  ab/abcdef1234...   (sha256 first 2 chars as prefix dir)
  cd/cdef5678...
```

Identical attachments (same sha256) share one physical file regardless of how many mails they appear in.

### D9. Configuration — single YAML file

One `config.yaml` holds everything, including the IMAP password. No environment variables, no permission checks, no keychain.

```yaml
# config.yaml
imap:
  host: imap.gmail.com
  port: 993
  user: austin.jan@gmail.com
  password: xxxxxxxxxxxxxxxx   # Gmail App Password
  folder: INBOX
defaults:
  since: 24h
database:
  path: ./mail-agent.db
attachments:
  dir: ./attachments
```

**Discipline**:
- `config.yaml` is git-ignored.
- `config.example.yaml` ships in git as the template (no real password).
- Gmail requires an App Password (2FA → app passwords), not the login password.

### D10. Error handling — best-effort, log everything

No fail-fast. Every error is logged and the run continues where possible. Exit code is always 0 (unless the program itself panics); log analysis decides what was wrong.

| Failure | Behaviour |
|---------|-----------|
| IMAP connection / auth failure | Log + exit cleanly; cron retries next tick |
| Transient network drop mid-fetch | Log + exit cleanly; cron retries next tick |
| Single mail parse failure | Log + skip that mail, continue |
| DB write failure | Log + continue |
| Attachment write failure | Log + mail still saved; attachment row marked failed |

**Logging format**: structured JSON via Go `log/slog`. One event per line, stable field names, easy to `grep` / `jq` / ship to a log store.

```json
{"time":"2026-04-22T14:32:10Z","level":"info","event":"fetch_start","since":"3d"}
{"time":"2026-04-22T14:32:12Z","level":"info","event":"mail_saved","uid":12345,"subject":"..."}
{"time":"2026-04-22T14:32:13Z","level":"warn","event":"mail_parse_failed","uid":12346,"error":"..."}
{"time":"2026-04-22T14:32:15Z","level":"info","event":"fetch_done","fetched":42,"skipped":1}
```

### D11. Project layout

```
mail-agent/
├── cmd/
│   └── mail-agent/
│       └── main.go              # CLI entrypoint, flag parsing, wires core
├── internal/
│   ├── core/
│   │   └── pipeline.go          # fetch → dedup → persist pipeline
│   ├── source/
│   │   ├── source.go            # Source interface
│   │   └── imap.go              # IMAPSource implementation
│   ├── store/
│   │   ├── store.go             # Store interface
│   │   └── sqlite.go            # SqliteStore implementation
│   ├── mail/
│   │   └── mail.go              # Mail, Attachment types
│   └── config/
│       └── config.go            # YAML parsing
├── config.example.yaml          # template, in git
├── config.yaml                  # real config (gitignored)
├── mail-agent.db                # SQLite file (gitignored)
├── attachments/                 # attachment files (gitignored)
├── go.mod
├── go.sum
└── README.md
```

**Why**:
- `cmd/` vs `internal/` — Go convention. `internal/` is not importable from outside the module, enforcing encapsulation.
- `source/` and `store/` each own an interface file and one implementation file. Adding `PostgresStore` or `GmailAPISource` later means one new file per package, no interface change.
- `core` imports only interfaces, never concrete `sqlite` or `imap` types.
- `mail` holds shared data types to prevent `source` ↔ `store` coupling.

### D12. CLI surface

MVP:
```bash
mail-agent read --since=3d                   # primary command
mail-agent read --since=3d --folder=INBOX    # override folder from config
mail-agent version
```

Future (post-MVP, post-#2):
```bash
mail-agent dispatch --mail-id=...            # Issue #2
mail-agent watch                             # future daemon mode
```

## Verification strategy — three layers

| Layer | Purpose | MVP expectation |
|-------|---------|-----------------|
| Manual smoke | Dev-loop sanity check | Required — run locally against a test mailbox |
| Unit (mock IMAP) | Pin down dedup, parsing, error paths | Required for core logic |
| Integration (live IMAP) | Catch real-server quirks | Optional — run before releases, not on every change |

**Acceptance criteria for MVP**:
1. Against a test mailbox with N mails in the `--since` window, the program logs exactly N mails saved on first run.
2. Re-running the same command logs 0 new mails (all deduplicated).
3. Sending a new mail and re-running logs exactly 1 new mail.
4. Stopping/restarting the process preserves dedup state across runs.
5. A mail with attachments writes all attachment files to disk with correct sha256-derived paths; the `attachments` table rows point to them.

## Architecture summary

```
┌──────────────────────────────────────────────────┐
│ cmd/mail-agent     (CLI entrypoint + cron glue)  │
└──────────────┬───────────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────────┐
│ internal/core       (time-range driven pipeline) │
│   fetch → dedup → persist → log                  │
└────┬──────────────────────────┬──────────────────┘
     │                          │
     ▼                          ▼
┌─────────────────┐      ┌───────────────────┐
│ internal/source │      │ internal/store    │
│   IMAPSource    │      │   SqliteStore     │
│   (swappable)   │      │   (swappable)     │
└─────────────────┘      └───────────────────┘
```

## References

- IMAP UID semantics: RFC 3501 §2.3.1.1
- Message-ID: RFC 5322 §3.6.4
- `emersion/go-imap/v2`: https://github.com/emersion/go-imap
- `modernc.org/sqlite` (pure-Go SQLite): https://pkg.go.dev/modernc.org/sqlite
