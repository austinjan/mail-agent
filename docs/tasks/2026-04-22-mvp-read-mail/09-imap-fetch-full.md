# Task 09: IMAPSource 完整抓取與 MIME Parse

**目標**：擴充 `IMAPSource.Fetch`，對 UID 清單執行完整 `UID FETCH`，抓取 RFC-822 原文與 flags，解析 MIME 後填入 `mail.Mail`，包含 headers、plain text、HTML body 與 attachments。

**依賴**：Task 08 已完成。

## 產出檔案

- Modify: `internal/source/imap.go`
- Modify: `internal/source/imap_test.go`
- Create: `internal/source/mime.go`
- Create: `internal/source/mime_test.go`
- Create: `internal/source/testdata/simple.eml`
- Create: `internal/source/testdata/with-attachment.eml`

## Steps

- [x] **Step 1: 準備 fixture**
- [x] **Step 2: 寫 parse 失敗測試 `internal/source/mime_test.go`**
- [x] **Step 3: 跑測試確認失敗**
- [x] **Step 4: 實作 `internal/source/mime.go`**
- [x] **Step 5: 跑測試確認通過**
- [x] **Step 6: 擴充 `IMAPSource.Fetch` 做完整抓取**
- [x] **Step 7: 擴充整合測試**
- [x] **Step 8: 跑整合測試**
- [x] **Step 9: Commit**

## 驗收

- `parseRFC822` 能從 fixture 正確解析 `BodyText`、`BodyHTML` 與 `Attachments`。
- base64 與 quoted-printable 內容會被正確 decode。
- `IMAPSource.Fetch` 會回傳完整 mail 資料。
- 同一天但早於 `since` 的 mail 會被本地精確過濾排除。