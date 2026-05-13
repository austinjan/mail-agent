# Task 17 — Semantic field extraction

**目標**：從 body / attachment text 中以語意方式萃取指定欄位，例如流量、揚程、材質、型號、數量、品牌、用途、備註。

**依賴**：Task 15、Task 16。

## 原則

- 不依賴固定關鍵字位置。
- 找不到就不輸出，不猜測。
- 每個結果都要有 `evidence_text`。
- 每個結果都要有 `confidence`。

## 驗收

- 可處理中英文混合敘述。
- 可從同一段文字抽出多個欄位。
- 結果寫入 `extracted_fields`。
