# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation layout

All plans, implementation details, and progress tracking live under `/docs`. Create the directory if it does not exist.

- `/docs/plans` — plans (design, approach, proposals)
- `/docs/tasks` — implementation items and progress status
- `/docs/references` — reference material

## Stack

- Language: Go 1.22+
- Module: `github.com/austinjan/mail-agent`
- IMAP client: `github.com/emersion/go-imap/v2`
- SQLite driver: `modernc.org/sqlite`（純 Go，無 CGO）
- YAML: `gopkg.in/yaml.v3`
- Logging: stdlib `log/slog`
- Testing: stdlib `testing`

## Project structure

```
cmd/mail-agent/        # CLI entrypoint（Task 11，待實作）
internal/
  config/              # Config 載入（Task 03，完成）
  mail/                # 共用資料型別（Task 02，完成）
  source/              # IMAP 抓信 + MIME 解析（Tasks 07–09，完成）
  store/               # SQLite 儲存（Tasks 04–06，待實作）
  core/                # Pipeline（Task 10，待實作）
docs/
  plans/               # 設計文件
  tasks/               # 實作 task 清單與進度
```

## Status

MVP Read Mail 進行中。Tasks 01–03、07–09 已完成；Tasks 04–06、10–12 待實作。
詳細進度見 `docs/tasks/2026-04-22-mvp-read-mail/README.md`。
