# Task 12 — 驗收與 smoke test

**目標**：對真實 Gmail / IMAP 信箱跑過 design 的五條 acceptance criteria，在此檔案記錄結果。

**依賴**：Task 11。

## 產出檔案

- Modify: 本檔（填入實際執行輸出）
- 不產出新程式碼；可能會根據發現補修 bug 並 commit 回前置 task。

## 前置

1. 複製 `config.example.yaml` 為 `config.yaml` 並填入 Gmail App Password。
2. 確保測試信箱在指定時間範圍內有數封 mail（其中至少一封含附件）。
3. 清空 DB 與 attachments：
   ```bash
   rm -f ./mail-agent.db ./mail-agent.db-journal
   rm -rf ./attachments
   ```

## 驗收步驟

- [ ] **AC-1: 首次 run 儲存 N 封**

```bash
go run ./cmd/mail-agent read --since=24h 2>&1 | tee run1.log | jq 'select(.event == "fetch_done")'
```

記錄 `fetched` 與 `saved` 兩個數字：

```
fetched = ____
saved   = ____
skipped = 0   ← 預期
```

交叉驗證：手動用 Gmail 看 24 小時內幾封 = N，程式 saved 應等於 N。

- [ ] **AC-2: 再 run 一次 → 0 saved、N skipped**

```bash
go run ./cmd/mail-agent read --since=24h 2>&1 | tee run2.log | jq 'select(.event == "fetch_done")'
```

記錄：

```
fetched = ____
saved   = 0   ← 預期
skipped = ____ (= AC-1 的 saved)
```

- [ ] **AC-3: 新寄一封 → 下次 run 恰好 +1**

從另一個帳號寄一封測試信到 testing 信箱，等它收到後：

```bash
go run ./cmd/mail-agent read --since=24h 2>&1 | tee run3.log | jq 'select(.event == "fetch_done")'
```

記錄：

```
fetched = AC-1 的 fetched + 1
saved   = 1
skipped = AC-1 的 saved
```

- [ ] **AC-4: 中斷再啟動 dedup 仍有效**

ctrl-C 中斷前一次 run 不容易測（run 很短），所以用 proxy：
1. 執行一次 run；
2. 不刪 DB；
3. 再 run → saved = 0。

等同 AC-2 但強調「DB 持久化」。檢查 `./mail-agent.db` 是否確實被保留：

```bash
ls -la ./mail-agent.db
sqlite3 ./mail-agent.db "SELECT COUNT(*) FROM mails;"
```

- [ ] **AC-5: 附件正確落地**

```bash
sqlite3 ./mail-agent.db \
  "SELECT a.filename, a.sha256, a.file_path FROM attachments a JOIN mails m ON a.mail_id = m.id LIMIT 5;"
```

對每列，確認實體檔案存在：

```bash
# 拿一筆的 file_path 做檢查，例如 b9/b94d27b9...
ls -la ./attachments/<前兩個字元>/<sha256>
# 驗證 sha256
shasum -a 256 ./attachments/<prefix>/<sha>
```

`shasum -a 256` 算出來的值要等於 `file_path` 裡的 sha256。

## 結果記錄表

| AC | 預期 | 實際 | 通過？ |
|----|------|------|--------|
| 1 | saved = N (手動計算) | | |
| 2 | saved = 0, skipped = N | | |
| 3 | saved = 1 | | |
| 4 | DB 持久化、再 run saved = 0 | | |
| 5 | sha256 一致、file_path 存在 | | |

## 發現的 bugs

（若有）列出需要回填修正的 task 與 commit。

## 結案 commit（無 code 變更則不 commit）

若本 task 過程中有修 bug，記得 commit 回對應 task 的檔案 + 重跑整棵測試：

```bash
go test ./...
```

全過後：

```bash
git add ...
git commit -m "修復 X：在 acceptance 測試發現"
```

## 驗收

- 表格五格全部打勾。
- 若有 bug，修復 commit 後 `go test ./...` 全過。
- 完成後把 `docs/tasks/2026-04-22-mvp-read-mail/README.md` 裡的 12 個 task checkbox 全部打勾。
