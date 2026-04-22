# Task 11 — CLI entrypoint

**目標**：在 `cmd/mail-agent/main.go` 串起所有元件。支援 `mail-agent read --since=3d [--folder=...] [--config=...]` 與 `mail-agent version`。

**依賴**：Task 03（config）、Task 06（store 完整版）、Task 09（imap 完整版）、Task 10（pipeline）。

## 產出檔案

- Modify: `cmd/mail-agent/main.go`
- Create: `cmd/mail-agent/main_test.go`（可選；主要靠 subcommand help 檢查）

## 設計筆記

- 用 stdlib `flag`，不引入 cobra / urfave 等。MVP 的 CLI 面向很小。
- `read` 是 subcommand；用最單純的手刻 dispatch（`os.Args[1]` 判斷）。
- Logger 一律用 `slog.NewJSONHandler(os.Stdout, ...)`，每個 run 輸出結構化 JSON。
- Exit code：依 design D10 — 永遠 `os.Exit(0)`（panic 除外）。Errors 只寫進 log。
- `version` 顯示靠 `runtime/debug.ReadBuildInfo()`（沒 version string 就顯示 `"dev"`）。

## Steps

- [ ] **Step 1: 寫 main.go**

```go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"github.com/austinjan/mail-agent/internal/config"
	"github.com/austinjan/mail-agent/internal/core"
	"github.com/austinjan/mail-agent/internal/source"
	"github.com/austinjan/mail-agent/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "read":
		runRead(os.Args[2:])
	case "version":
		fmt.Println(versionString())
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `mail-agent — fetch mails from IMAP into local storage

Usage:
  mail-agent read --since=<duration> [--folder=INBOX] [--config=./config.yaml]
  mail-agent version

Examples:
  mail-agent read --since=3d
  mail-agent read --since=24h --folder=INBOX
  mail-agent read --since=2026-04-01T00:00:00Z --config=./config.yaml
`)
}

func runRead(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	var (
		sinceStr = fs.String("since", "", "required: 3d | 1w | 24h | RFC-3339 timestamp")
		folder   = fs.String("folder", "", "IMAP folder; overrides config")
		cfgPath  = fs.String("config", "config.yaml", "path to YAML config")
	)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("", "event", "config_load_failed", "error", err.Error())
		return
	}

	effectiveSince := *sinceStr
	if effectiveSince == "" {
		effectiveSince = cfg.Defaults.Since
	}
	if effectiveSince == "" {
		logger.Error("", "event", "since_missing", "hint", "pass --since=... or set defaults.since in config")
		return
	}
	since, err := source.ParseSince(effectiveSince, time.Now().UTC())
	if err != nil {
		logger.Error("", "event", "since_parse_failed", "input", effectiveSince, "error", err.Error())
		return
	}

	effectiveFolder := *folder
	if effectiveFolder == "" {
		effectiveFolder = cfg.IMAP.Folder
	}
	if effectiveFolder == "" {
		effectiveFolder = "INBOX"
	}

	st, err := store.OpenSQLite(cfg.Database.Path, cfg.Attachments.Dir)
	if err != nil {
		logger.Error("", "event", "store_open_failed", "error", err.Error())
		return
	}
	defer st.Close()

	src := source.NewIMAPSource(source.IMAPConfig{
		Host:     cfg.IMAP.Host,
		Port:     cfg.IMAP.Port,
		User:     cfg.IMAP.User,
		Password: cfg.IMAP.Password,
	})

	p := core.New(src, st, logger)
	if _, err := p.Run(effectiveFolder, since); err != nil {
		logger.Error("", "event", "pipeline_error", "error", err.Error())
		// design D10: exit 0 anyway
		return
	}
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
```

- [ ] **Step 2: 編譯**

```bash
go build ./...
```

預期：無錯誤；產出 `./mail-agent`（若跑 `go build -o mail-agent ./cmd/mail-agent`）。

- [ ] **Step 3: 煙霧測試 help / version**

```bash
go run ./cmd/mail-agent
# 輸出 usage；exit code 2

go run ./cmd/mail-agent version
# 輸出 "dev" 或實際 version

go run ./cmd/mail-agent read
# log 事件 since_missing（因為沒帶 --since 也沒 config）

go run ./cmd/mail-agent read --since=3d --config=/nonexistent.yaml
# log 事件 config_load_failed
```

- [ ] **Step 4: （可選）加 smoke test**

`cmd/mail-agent/main_test.go`（只測 version 組裝）：

```go
package main

import "testing"

func TestVersionStringNotEmpty(t *testing.T) {
	if versionString() == "" {
		t.Error("versionString should never return empty")
	}
}
```

```bash
go test ./cmd/mail-agent/...
```

預期：PASS。

- [ ] **Step 5: Commit**

```bash
git add cmd
git commit -m "新增 mail-agent CLI entrypoint"
```

## 驗收

- `go build ./...` 成功。
- 不帶 `--since` 且 config 無 default 時，程式 log `since_missing` 並正常結束（exit 0）。
- `mail-agent version` 輸出版本字串。
- 所有子命令完成後程序 exit 0（除非 Go runtime panic）。
