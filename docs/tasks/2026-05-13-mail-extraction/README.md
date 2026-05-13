# Mail Extraction 實作計畫索引

- Design: [../../plans/2026-05-13-mail-extraction-design.md](../../plans/2026-05-13-mail-extraction-design.md)
- Builds on: MVP Read Mail Tasks 01-12
- Constraint: 不使用 OCR

## Task 清單

| # | Task | 產出 | 依賴 |
|---|------|------|------|
| 13 | [Extraction schema](13-extraction-schema.md) | `extraction_jobs`、`extracted_fields` schema 與測試 | 12 |
| 14 | [Extract CLI 骨架](14-extract-cli-skeleton.md) | `mail-agent extract enqueue/run/show` skeleton | 13 |
| 15 | [Mail body text extraction](15-mail-body-extraction.md) | 從 `mails.body_text/body_html` 建立 body jobs 並萃取文字 | 14 |
| 16 | [Attachment text extraction without OCR](16-attachment-text-extraction.md) | PDF / Excel / Word 文字擷取；image-only 標為 unsupported | 14 |
| 17 | [Semantic field extraction](17-semantic-field-extraction.md) | 流量、揚程、材質等欄位萃取與 evidence | 15, 16 |
| 18 | [Extraction result review](18-extraction-result-review.md) | 查詢與人工檢查指令 | 17 |

## 目前進度

- [x] Task 13
- [x] Task 14
- [x] Task 15
- [x] Task 16
- [x] Task 17
- [x] Task 18

## 實作備註

- `mail-agent extract enqueue --since=24h` 會為已入庫 mail body 與附件建立 extraction jobs。
- `mail-agent extract run --limit=20` 預設使用 Gemini LLM 處理 pending jobs，寫入 `extracted_fields`。
- `mail-agent extract run --mode=rules --limit=20` 可使用本機規則備援。
- `mail-agent extract show --mail-id=123` 會列出欄位、值、單位、confidence、evidence 與來源。
- `mail-agent extract export --out=extracted_fields.csv` 會把萃取結果匯出成 Excel 友善的 TypeSearch CSV。
- 不使用 OCR；圖片、掃描 PDF 或無文字層 PDF 會標為 `unsupported`。
