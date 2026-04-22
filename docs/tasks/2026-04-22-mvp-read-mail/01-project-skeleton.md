# Task 01 — 專案骨架

**目標**：建立 Go module、目錄結構、`.gitignore` 規則，讓 `go build ./...` 在空的專案上能過。

**依賴**：無。

## 產出檔案

- Create: `go.mod`
- Create: `cmd/mail-agent/main.go`（空 `main()`，能編譯通過）
- Create: `internal/core/.gitkeep`
- Create: `internal/source/.gitkeep`
- Create: `internal/store/.gitkeep`
- Create: `internal/mail/.gitkeep`
- Create: `internal/config/.gitkeep`
- Modify: `.gitignore`

## Steps

- [ ] **Step 1: 初始化 Go module**

```bash
cd /Users/austinjan/code/mail-agent
go mod init github.com/austinjan/mail-agent
```

預期：產生 `go.mod`，內容包含 `module github.com/austinjan/mail-agent` 與 `go 1.22`（或當前 Go 版本）。

- [ ] **Step 2: 擴充 `.gitignore`**

把下列內容附加到 `.gitignore`：

```
# Build output
/mail-agent
/bin/

# Local config（含 IMAP password）
/config.yaml

# SQLite DB
*.db
*.db-journal

# Attachments
/attachments/

# macOS
.DS_Store
```

- [ ] **Step 3: 建立 package 目錄與空 `main.go`**

```bash
mkdir -p cmd/mail-agent internal/core internal/source internal/store internal/mail internal/config
touch internal/core/.gitkeep internal/source/.gitkeep internal/store/.gitkeep internal/mail/.gitkeep internal/config/.gitkeep
```

`cmd/mail-agent/main.go`：

```go
package main

func main() {}
```

- [ ] **Step 4: 確認 build 可以過**

```bash
go build ./...
```

預期：無輸出、exit 0。

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd internal .gitignore
git commit -m "初始化 Go 專案骨架"
```

## 驗收

- `go build ./...` 無錯誤。
- `ls internal/` 看到五個子目錄。
- `.gitignore` 會擋掉 `config.yaml`、`*.db`、`attachments/`。
