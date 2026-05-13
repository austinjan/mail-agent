# Mail Extraction — Design

- Date: 2026-05-13
- Status: Proposed for implementation
- Builds on: [MVP: Read Mail — Design](./2026-04-22-mvp-read-mail-design.md)

## Goal

After mails and attachments are stored locally, extract requested business data
from unpredictable mail and attachment formats. Target fields include, but are
not limited to, flow rate, head, material, model, quantity, brand, application,
and free-form notes.

The extractor must not assume a fixed keyword position. Values may appear in
paragraphs, tables, mixed Chinese/English text, or attachment content.

## Non-goals

- OCR is explicitly out of scope. Image-only PDFs and image attachments are
  recorded as unsupported.
- The read-mail pipeline should not run extraction inline. Reading mail and
  extracting data remain separate commands and failure domains.
- The first extraction pass does not need to decide final business actions.

## Pipeline

```text
mails / attachments
        |
        v
extract jobs
        |
        v
body / attachment text extraction
        |
        v
chunking + semantic extraction
        |
        v
extracted fields with evidence
        |
        v
review / dispatch / downstream automation
```

## Data Model

`extraction_jobs` tracks work and retry state.

- `id`
- `mail_id`
- `attachment_id`, nullable for mail-body jobs
- `source_type`: `body`, `attachment`
- `status`: `pending`, `running`, `done`, `failed`, `unsupported`
- `attempts`
- `error`
- `created_at`
- `updated_at`
- `finished_at`

`extracted_fields` stores normalized results and traceable evidence.

- `id`
- `job_id`
- `mail_id`
- `attachment_id`, nullable
- `field_name`, for example `流量`
- `field_value`, for example `120`
- `unit`, for example `m3/h`
- `confidence`, numeric 0 to 1
- `evidence_text`, the quoted source snippet supporting the value
- `source_type`: `body`, `attachment`
- `source_label`, for example filename or mail body
- `created_at`

The schema is intentionally flexible because field names are not limited to a
fixed pump spec list.

## Extraction Strategy

1. Convert source content to text or table-like text. Mail body is first. PDF,
   Excel, and Word attachment extraction are added as separate tasks. OCR is not
   used.
2. Chunk content by paragraph, table row, or page-like boundaries while keeping
   source labels.
3. Ask semantic extraction to identify requested fields even when labels are
   absent, translated, abbreviated, or far from values.
4. Require each extracted value to include evidence text. Missing values are
   stored as no result rather than guessed.
5. Persist extracted fields and job status so extraction can be retried without
   duplicating work.

## Field Scope

Initial field targets:

- `流量`
- `揚程`
- `材質`
- `型號`
- `數量`
- `品牌`
- `用途`
- `備註`

The field list should be configurable later. T17 may start with a hard-coded
default list to keep the first implementation small.

## Attachment Support Order

1. Mail body text and HTML converted to text.
2. Text-based PDF extraction.
3. Excel workbook extraction.
4. Word document extraction.
5. Unsupported binary/image-only attachments are marked `unsupported`.

OCR is intentionally excluded from this roadmap.

## Error Handling

- One failed job must not block other jobs.
- Retriable failures increment `attempts` and store `error`.
- Unsupported formats are marked `unsupported`, not `failed`.
- Extraction should be safe to rerun.

## CLI Surface

Planned commands:

```bash
mail-agent extract enqueue --since=24h
mail-agent extract run --limit=20
mail-agent extract show --mail-id=123
```

## Acceptance

- Existing read-mail behavior remains unchanged.
- Extraction schema exists after `OpenSQLite`.
- Jobs can represent body and attachment work independently.
- Extracted values include evidence and confidence.
- OCR is not required and not invoked.
