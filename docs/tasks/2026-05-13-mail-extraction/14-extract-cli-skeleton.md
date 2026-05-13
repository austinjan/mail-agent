# Task 14 — Extract CLI 骨架

**目標**：新增 `mail-agent extract` 子命令骨架，但先不做實際語意萃取。

**依賴**：Task 13。

**Last commit message**：`完成 extraction pipeline`

## 預計指令

```bash
mail-agent extract enqueue --since=24h
mail-agent extract run --limit=20
mail-agent extract show --mail-id=123
```

## 驗收

- [x] CLI help 可看到 `extract`。
- [x] `extract enqueue`、`extract run`、`extract show` 已接到 CLI。
- [x] 不影響既有 `read` 與 `version`。
