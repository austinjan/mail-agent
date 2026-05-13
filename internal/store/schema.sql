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

CREATE TABLE IF NOT EXISTS extraction_jobs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    mail_id        INTEGER NOT NULL REFERENCES mails(id),
    attachment_id  INTEGER REFERENCES attachments(id),
    source_type    TEXT NOT NULL CHECK (source_type IN ('body', 'attachment')),
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'running', 'done', 'failed', 'unsupported')),
    attempts       INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    error          TEXT,
    created_at     TIMESTAMP NOT NULL,
    updated_at     TIMESTAMP NOT NULL,
    finished_at    TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_extraction_jobs_body_unique
ON extraction_jobs(mail_id, source_type)
WHERE attachment_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_extraction_jobs_attachment_unique
ON extraction_jobs(attachment_id, source_type)
WHERE attachment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_extraction_jobs_status ON extraction_jobs(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_extraction_jobs_mail_id ON extraction_jobs(mail_id);

CREATE TABLE IF NOT EXISTS extracted_fields (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id         INTEGER NOT NULL REFERENCES extraction_jobs(id),
    mail_id        INTEGER NOT NULL REFERENCES mails(id),
    attachment_id  INTEGER REFERENCES attachments(id),
    field_name     TEXT NOT NULL,
    field_value    TEXT NOT NULL,
    unit           TEXT,
    confidence     REAL CHECK (confidence >= 0.0 AND confidence <= 1.0),
    evidence_text  TEXT NOT NULL,
    source_type    TEXT NOT NULL CHECK (source_type IN ('body', 'attachment')),
    source_label   TEXT,
    created_at     TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_extracted_fields_mail_id ON extracted_fields(mail_id);
CREATE INDEX IF NOT EXISTS idx_extracted_fields_job_id ON extracted_fields(job_id);
CREATE INDEX IF NOT EXISTS idx_extracted_fields_field_name ON extracted_fields(field_name);
