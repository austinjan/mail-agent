# Task 15 — Mail body text extraction

**目標**：從已入庫的 `mails.body_text` / `mails.body_html` 建立 body extraction jobs，並提供乾淨文字給後續語意萃取。

**依賴**：Task 14。

## 重點

- Body job 的 `attachment_id` 為 NULL。
- HTML 轉文字，不直接拿 HTML tag 給後續萃取。
- 不假設關鍵字位置。

## 驗收

- 可為未處理 mail 建立 body jobs。
- 重跑不重複建立同一 mail body job。
- 空 body 可標記 unsupported 或 done with no fields。
