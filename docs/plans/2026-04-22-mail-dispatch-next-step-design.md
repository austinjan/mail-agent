# Mail Dispatch: 下一步處理設計（v1）

- Date: 2026-04-22
- Status: Proposed v1
- Related:
  - [#1 讀取信件](https://github.com/austinjan/mail-agent/issues/1)
  - [#2 Dispatch task based on mail](https://github.com/austinjan/mail-agent/issues/2)
  - [MVP: Read Mail — Design](./2026-04-22-mvp-read-mail-design.md)

## 背景

Issue #1 的 MVP 已定義「讀信 -> dedup -> 落庫 -> 記錄 log」。

Issue #2 要補上的，是一封新信成功 ingest 之後，系統如何挑出需要後續處理的信，建立工作，執行處理，並留下結果。

這份文件只定義 **dispatch v1**，目標是先把最小但完整的 pipeline 做對。

## v1 目標

v1 要解決的問題很單純：

1. 從已落庫的 `mails` 中挑出尚未 dispatch 的信
2. 為合格信件建立一筆 work item
3. 執行一種處理：`summarize_mail`
4. 將結果寫回 SQLite
5. 確保重跑時不會重複建立同類 work item
6. 在 crash 或中途中斷後可恢復

## v1 非目標

以下明確不放進 v1：

- `extract_data`
- `draft_reply`
- 人工審核流程
- 多種 work item kinds
- `dispatch_runs` 歷史表
- 外部系統寫入
- webhook / JSON / Markdown 額外輸出
- LLM-specific intent / confidence / entities schema

## 核心決策

### D1. `read` 與 `dispatch` 分離

`read` 只負責：

- 連 IMAP
- 抓信
- dedup
- 存到 `mails` / `attachments`

`dispatch` 只負責：

- 挑選尚未處理的 mail
- 建立 `work_items`
- 執行 `summarize_mail`
- 記錄結果

原因：

- ingest 與後處理失敗可以分開觀察與重跑
- 比較符合 one-shot CLI + cron 的模型
- 後續若加入背景 worker，不需重寫 `read`

v1 CLI：

```bash
mail-agent read --since=5m
mail-agent dispatch --limit=20
```

### D2. v1 只有一種 work item：`summarize_mail`

v1 只實作 `summarize_mail`。

原因：

- 足以驗證完整 dispatch pipeline
- 不需要先定義特定資料抽取 schema
- 可以避免 `extract_data` 範圍過大、定義不清

### D3. v1 不做獨立 Interpretation / Review phase

v1 的處理流程直接收斂成：

```text
gate -> create work item -> execute -> record outcome
```

原因：

- 只有一種 work item 時，不需要再做 intent resolution
- v1 沒有高風險副作用，不需要 review gate

未來若加入第二種 kind 或人工審核，再把 phase 展開。

### D4. v1 沿用 MVP 的錯誤處理哲學

dispatch v1 與 read MVP 一致：

- best-effort
- 記錄 structured log
- 能繼續就繼續
- 程式正常結束時 exit code 為 0

例：

- 某封信建立 work item 失敗 -> log + 繼續下一封
- 某筆摘要生成失敗 -> work item 標 `failed`，log 後繼續
- 沒有可 dispatch 的信 -> log summary，正常結束

### D5. v1 的 LLM provider 與 model

v1 的摘要能力使用 Gemini。

預設模型：

- `gemini-3.1-flash-lite-preview`

建議 fallback：

- `gemini-2.5-flash`

原因：

- `gemini-3.1-flash-lite-preview` 成本低、吞吐高，適合高頻的信件摘要工作
- v1 先做 `summarize_mail`，任務相對單純，適合先用 flash-lite 類模型
- 由於這是 preview model，保留一個較穩定的 fallback 比較安全

建議 config：

```yaml
llm:
  provider: gemini
  model: gemini-3.1-flash-lite-preview
  fallback_model: gemini-2.5-flash
```

## v1 資料模型

v1 只使用：

- `mails`
- `work_items`

不建立 `dispatch_runs`。

### work_items

建議欄位：

- `id`
- `mail_id`
- `kind`
- `status`
- `attempt_count`
- `last_error`
- `result_payload`
- `created_at`
- `updated_at`

狀態值：

- `pending`
- `running`
- `succeeded`
- `failed`

### 關鍵約束

v1 必須在 schema 層保證 idempotency：

```sql
UNIQUE (mail_id, kind)
```

原因：

- 驗收條件要求同一封信重跑時不能重複建立同類 work item
- 這個保證不能只靠應用程式判斷，必須落在資料庫約束上

## v1 選信規則

dispatch v1 只處理：

- 已存在於 `mails`
- 尚未有 `summarize_mail` work item 的信

選信 query 概念如下：

```sql
SELECT m.id
FROM mails m
LEFT JOIN work_items w
  ON w.mail_id = m.id
 AND w.kind = 'summarize_mail'
WHERE w.id IS NULL
ORDER BY m.received_at ASC, m.id ASC
LIMIT ?;
```

說明：

- `LEFT JOIN ... WHERE w.id IS NULL` 明確表示「還沒建立過同類 work item」
- `ORDER BY received_at ASC, id ASC` 讓行為穩定且可預期
- `LIMIT` 控制單次 dispatch 負載

v1 不在這一版引入複雜 eligibility 規則。預設策略是：

- 只要 mail 已成功 ingest
- 且尚未建立 `summarize_mail`
- 就可進入 dispatch

未來若需要 sender/domain/folder/subject gating，再額外加規則。

## v1 執行流程

每次 `mail-agent dispatch --limit=N` 的流程如下：

1. 先將過久的 `running` work item 重設為 `pending`
2. 用選信 query 找出最多 N 封尚未建立 `summarize_mail` 的 mails
3. 對每封 mail 嘗試建立一筆 `work_items(kind='summarize_mail', status='pending')`
4. 逐筆挑選 `pending` work item 執行摘要
5. 執行前將狀態改為 `running`，並更新 `attempt_count`
6. 成功則寫入 `result_payload` 並標成 `succeeded`
7. 失敗則寫入 `last_error` 並標成 `failed`
8. 輸出本次 dispatch summary log

## Crash Recovery 與 Retry

v1 必須定義 `running` 卡死時的恢復方式。

### stuck-running recovery

在每次 dispatch 開始時，先執行：

- 將 `status='running'`
- 且 `updated_at < now() - 10 minutes`

的 work item 重設為：

- `status='pending'`
- `last_error='reset from stale running state'`

原因：

- one-shot CLI 在 crash 時不會自動清理 `running`
- 若不重設，work item 可能永遠卡住

### retry policy

v1 的 retry policy 採最簡單版本：

- `failed` work item 不自動重試
- 只有被 reset 的 stale `running` 會重新進入 `pending`

原因：

- 避免 v1 一開始就引入 backoff / max attempts / retry queue 複雜度
- 先把 crash recovery 做明確

未來若需要自動重試，再新增：

- `next_attempt_at`
- backoff
- max attempts

## v1 結果格式

`result_payload` 先用 JSON 字串儲存。

`summarize_mail` 的 v1 結果格式：

```json
{
  "summary": "string"
}
```

v1 不定義更複雜的 schema。

建議摘要內容至少包含：

- 這封信的主旨重點
- 寄件者
- 是否需要後續動作
- 重要時間、附件或要求

例：

```json
{
  "summary": "Alice 寄來專案進度更新，表示 API schema 已完成，請在本週五前確認是否進入測試，並附上一份規格文件。"
}
```

## LLM 呼叫策略

v1 建議由應用程式組出固定 prompt，將以下欄位餵給模型：

- `subject`
- `from_addr`
- `to_addrs`
- `cc_addrs`
- `received_at`
- `body_text`
- 附件檔名列表

若 `body_text` 為空，可退回使用其他可用文字內容。

v1 建議要求模型輸出 structured JSON，對應：

```json
{
  "summary": "string"
}
```

若主模型失敗，可採以下策略：

1. 先記錄錯誤
2. 依設定改用 `fallback_model`
3. fallback 仍失敗則將 work item 標記為 `failed`

v1 不要求自動多次重試不同 prompt，只需單次 fallback 即可。

## 驗收條件

1. 新信 ingested 後，`dispatch` 可以選到該信並建立一筆 `summarize_mail` work item
2. 同一封信重跑 `dispatch` 不會重複建立第二筆 `summarize_mail` work item
3. `summarize_mail` 成功後，work item 狀態為 `succeeded`，且 `result_payload` 含摘要
4. `summarize_mail` 失敗後，work item 狀態為 `failed`，且 `last_error` 有內容
5. 若程序在 `running` 狀態中崩潰，超過 10 分鐘後下一次 dispatch 會將該 work item 重設為 `pending`
6. 全程採 best-effort；單筆失敗不應中止整次 dispatch

## 未來擴充方向

以下保留到 v2 之後：

- `extract_data`
- 第二種以上的 `kind`
- sender / domain / folder / subject gating
- `dispatch_runs` 歷史
- 人工審核狀態
- 自動重試與 backoff
- 外部副作用

## 下一步建議

這份設計若進入 implementation planning，建議拆成以下 task：

1. 定義 `work_items` schema 與 `UNIQUE (mail_id, kind)`
2. 實作 dispatch selection query
3. 實作 stale `running` reset
4. 實作 `summarize_mail` work item 狀態機
5. 定義 `result_payload` 的最小 JSON 格式
6. 補齊 dispatch 的測試與驗收案例
