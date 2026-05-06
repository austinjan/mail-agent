# Task 08: IMAPSource 連線與 SEARCH SINCE

**目標**：實作 `IMAPSource` 的第一階段：使用 `emersion/go-imap/v2` 連線到 IMAP over TLS、登入、選擇 folder、讀取 UIDVALIDITY，並用 `UID SEARCH SINCE` 取得候選 UID 清單。

**依賴**：Task 07 已完成。

## 產出檔案

- Create: `internal/source/imap.go`
- Create: `internal/source/imap_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

## Steps

- [x] **Step 1: 加入 go-imap 依賴**
- [x] **Step 2: 實作 `internal/source/imap.go` 骨架**
- [x] **Step 3: 寫整合測試 `internal/source/imap_test.go`**
- [x] **Step 4: 確認 unit 測試通過**
- [x] **Step 5: 本地實際跑一次整合測試**
- [x] **Step 6: Commit**

## 驗收

- 單元測試不需要網路即可通過。
- 整合測試在提供 IMAP credentials 時能取得非零 `uidValidity`。
- `IMAPSource` 滿足 `Source` 介面。