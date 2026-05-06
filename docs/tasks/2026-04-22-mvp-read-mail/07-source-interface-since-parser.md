# Task 07: Source 介面與 since 時間解析

**目標**：定義 `Source` 介面，讓核心 pipeline 能依賴抽象來源；同時實作 `ParseSince`，將 `3d`、`1w`、`24h` 或 RFC-3339 時間字串轉成 `time.Time`。

**依賴**：Task 02 已完成，已有 `mail.Mail` 型別可用。

## 產出檔案

- Create: `internal/source/source.go`
- Create: `internal/source/since.go`
- Create: `internal/source/since_test.go`

## Steps

- [x] **Step 1: 定義介面 `internal/source/source.go`**
- [x] **Step 2: 寫 since parser 失敗測試**
- [x] **Step 3: 跑測試確認失敗**
- [x] **Step 4: 實作 `internal/source/since.go`**
- [x] **Step 5: 跑測試確認通過**
- [x] **Step 6: Commit**

## 驗收

- `go test ./internal/source/...` 通過。
- RFC-3339 與相對時間都能正確解析。
- invalid cases 會回傳 error。
- `Source` 介面已就位，可供 IMAP source 與 core pipeline 使用。