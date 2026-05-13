# Task 18 — Extraction result review

**目標**：提供查詢與人工檢查萃取結果的指令，讓使用者能回看來源 mail、附件、欄位、evidence 與 confidence。

**依賴**：Task 17。

**Last commit message**：`完成 extraction pipeline`

## 預計指令

```bash
mail-agent extract show --mail-id=123
mail-agent extract show --job-id=456
mail-agent extract export --out=extracted_fields.csv
```

## 驗收

- [x] 可列出指定 mail 的所有萃取欄位。
- [x] 可看到 evidence 與來源附件檔名。
- [x] 可由 extraction job 狀態追蹤 unsupported / failed 原因。
- [x] 可匯出 TypeSearch CSV，供沒有 DB 工具時用 Excel 檢視。
