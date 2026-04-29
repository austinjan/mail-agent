CREATE TABLE IF NOT EXISTS mails (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    uid_validity  INTEGER NOT NULL,
    uid           INTEGER NOT NULL,
    folder        TEXT NOT NULL,
    message_id    TEXT,
    subject       TEXT,
    from_addr     TEXT,
    to_addrs      TEXT,
    cc_addrs      TEXT,
    reply_to      TEXT,
    in_reply_to   TEXT,
    refs          TEXT,
    flags         TEXT,
    received_at   TIMESTAMP,
    body_text     TEXT,
    body_html     TEXT,
    raw_headers   TEXT,
    fetched_at    TIMESTAMP NOT NULL,
    UNIQUE (uid_validity, uid, folder)
);

CREATE INDEX IF NOT EXISTS idx_mails_message_id ON mails(message_id);

CREATE TABLE IF NOT EXISTS attachments (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    mail_id       INTEGER NOT NULL REFERENCES mails(id),
    filename      TEXT,
    content_type  TEXT,
    size_bytes    INTEGER,
    sha256        TEXT NOT NULL,
    file_path     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attachments_mail_id ON attachments(mail_id);
CREATE INDEX IF NOT EXISTS idx_attachments_sha256 ON attachments(sha256);
