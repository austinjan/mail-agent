# Task 14 — Extract CLI 骨架

**目標**：新增 `mail-agent extract` 子命令骨架，但先不做實際語意萃取。

**依賴**：Task 13。

## 預計指令

```bash
mail-agent extract enqueue --since=24h
mail-agent extract run --limit=20
mail-agent extract show --mail-id=123
```

## 驗收

- CLI help 可看到 `extract`。
- 未實作路徑給出清楚 log 或錯誤訊息。
- 不影響既有 `read` 與 `version`。
