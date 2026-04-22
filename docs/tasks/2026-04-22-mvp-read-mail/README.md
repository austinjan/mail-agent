# MVP: Read Mail — 實作計畫索引

- Design: [../../plans/2026-04-22-mvp-read-mail-design.md](../../plans/2026-04-22-mvp-read-mail-design.md)
- Branch: `feat/mvp-read-mail-impl-plan`
- Module path: `github.com/austinjan/mail-agent`
- Go version: 1.22+

## 展開原則

- 每個 task 是一個可獨立完成 + 可 commit 的工作單位（大約 30–90 分鐘）。
- 每個 task 走 TDD：先失敗測試 → 最小實作 → 測試過 → commit。
- 依賴順序就是檔案編號順序；編號靠前的 task 不依賴後面的。
- `core` 套件只認識介面（`source.Source` / `store.Store`），不認識具體實作。
- 所有 commit 訊息用簡短中文。

## 技術選型（已由 design 決定）

| 角色 | 套件 |
|------|------|
| IMAP client | `github.com/emersion/go-imap/v2` |
| SQLite driver | `modernc.org/sqlite`（純 Go，無 CGO） |
| YAML | `gopkg.in/yaml.v3` |
| Logging | stdlib `log/slog`（JSON handler） |
| Testing | stdlib `testing`（不引入 testify，降低外部依賴） |

## Task 清單

| # | Task | 產出 | 依賴 |
|---|------|------|------|
| 01 | [專案骨架](01-project-skeleton.md) | `go.mod`、目錄結構、`.gitignore` | — |
| 02 | [Mail 資料型別](02-mail-types.md) | `internal/mail/mail.go` | 01 |
| 03 | [Config 載入](03-config.md) | `internal/config`、`config.example.yaml` | 01 |
| 04 | [Store 介面 + SQLite schema](04-store-interface-schema.md) | `internal/store/store.go`、`schema.sql`、`sqlite.go` 骨架 | 02 |
| 05 | [Store: SaveMail + HasSeen](05-store-save-mail-dedup.md) | `sqlite.go` 新增寫入與 dedup | 04 |
| 06 | [Store: SaveAttachment + content-hashed 檔案](06-store-attachments.md) | `sqlite.go` 新增 attachment 邏輯 | 05 |
| 07 | [Source 介面 + since 時間解析](07-source-interface-since-parser.md) | `internal/source/source.go`、since parser | 02 |
| 08 | [IMAPSource: 連線 + SEARCH SINCE](08-imap-connect-search.md) | `internal/source/imap.go` 第一階段 | 07 |
| 09 | [IMAPSource: 完整抓取 + parse](09-imap-fetch-full.md) | `internal/source/imap.go` 第二階段 | 08 |
| 10 | [Core pipeline + slog](10-core-pipeline.md) | `internal/core/pipeline.go` | 05, 07 |
| 11 | [CLI entrypoint](11-cli-entrypoint.md) | `cmd/mail-agent/main.go` | 03, 06, 09, 10 |
| 12 | [驗收與 smoke test](12-acceptance-verification.md) | 對活信箱跑過五條 acceptance 條件 | 11 |

## 驗收條件（來自 design §Verification）

1. 信箱在 `--since` 窗內有 N 封，首次執行 log 到 N 封 saved。
2. 同命令再跑一次，log 到 0 封 new（全部 deduplicated）。
3. 寄一封新信再跑，log 到 1 封 new。
4. 中斷程序再啟動，dedup 狀態仍然保留。
5. 含附件的信 → 附件落地到 `attachments/<ab>/<sha256>` 且 `attachments` 資料表有對應列。

## 目前進度

- [ ] Task 01
- [ ] Task 02
- [ ] Task 03
- [ ] Task 04
- [ ] Task 05
- [ ] Task 06
- [ ] Task 07
- [ ] Task 08
- [ ] Task 09
- [ ] Task 10
- [ ] Task 11
- [ ] Task 12
