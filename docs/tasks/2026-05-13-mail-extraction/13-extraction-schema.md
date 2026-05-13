# Task 13 — Extraction schema

**目標**：新增資料萃取所需的 SQLite schema，讓後續可以追蹤 mail body 與 attachment 的萃取工作、重試狀態、萃取結果、evidence 與 confidence。

**依賴**：Task 12 已完成。

**Last commit message**：`新增 extraction schema`

## 產出檔案

- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_test.go`
- Create: `docs/plans/2026-05-13-mail-extraction-design.md`
- Create: `docs/tasks/2026-05-13-mail-extraction/README.md`

## 設計筆記

- `extraction_jobs` 管理待處理、處理中、完成、失敗、unsupported。
- `extracted_fields` 保存欄位名稱、值、單位、confidence、evidence。
- `attachment_id` 可為 NULL，代表 mail body job。
- 不使用 OCR；image-only/binary 內容後續應標成 `unsupported`。

## Steps

- [x] **Step 1: 寫 schema 測試**
- [x] **Step 2: 新增 `extraction_jobs`**
- [x] **Step 3: 新增 `extracted_fields`**
- [x] **Step 4: 建立常用 index**
- [x] **Step 5: 跑測試**

## 驗收

- `go test ./internal/store/...` 通過。
- `OpenSQLite` 後可查到 `extraction_jobs` 與 `extracted_fields`。
- `extracted_fields.confidence` 限制在 0 到 1。
- schema 不影響既有 mails / attachments 行為。
