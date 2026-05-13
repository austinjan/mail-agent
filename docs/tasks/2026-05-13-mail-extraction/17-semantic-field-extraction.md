# Task 17 — Semantic field extraction

**目標**：從 body / attachment text 中以語意方式萃取指定欄位，例如流量、揚程、材質、型號、數量、品牌、用途、備註。

**依賴**：Task 15、Task 16。

**Last commit message**：`完成 extraction pipeline`

## 原則

- 不依賴固定關鍵字位置。
- 找不到就不輸出，不猜測。
- 每個結果都要有 `evidence_text`。
- 每個結果都要有 `confidence`。

## 驗收

- [x] 可處理中英文混合敘述。
- [x] 可從同一段文字抽出多個欄位。
- [x] 結果寫入 `extracted_fields`。

## 目前策略

- 預設使用 OpenAI LLM structured output 萃取欄位。
- 本機規則保留為 `--mode=rules` 備援。
- 欄位位置不固定時，交由 LLM 依整段文字與 evidence 判斷。
- LLM 輸出依 TypeSearch 前 8 欄結構：`Item`、`CMH`、`m`、`RPM`、`黏度`、`比重`、`SSVP管長`、`機殼鑄造方式`。
