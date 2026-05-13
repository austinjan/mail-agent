# Task 16 — Attachment text extraction without OCR

**目標**：從附件轉出文字或表格型文字，供後續語意萃取使用。明確不使用 OCR。

**依賴**：Task 14。

**Last commit message**：`完成 extraction pipeline`

## 支援順序

- Text-based PDF
- Excel workbook
- Word document
- Plain text / CSV

## 不支援

- Image-only PDF
- 圖片附件
- 需要 OCR 才能讀取的掃描文件

## 驗收

- [x] 支援格式可產生 extraction input text。
- [x] 不支援格式標為 `unsupported`，不算程式錯誤。
- [x] 不呼叫 OCR 工具或服務。

## 目前限制

- PDF 只嘗試擷取既有文字層，不處理掃描圖像。
- 圖片附件與 image-only PDF 會標為 `unsupported`。
