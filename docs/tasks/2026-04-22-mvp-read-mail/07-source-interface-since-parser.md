# Task 07 — Source 介面 + since 時間解析

**目標**：定義 `Source` 介面（讓 IMAPSource 可被 mock），並實作 `ParseSince`，把字串（`3d`/`1w`/`24h`/`2026-04-01T00:00:00Z`）轉成 `time.Time`。

**依賴**：Task 02。

## 產出檔案

- Create: `internal/source/source.go`
- Create: `internal/source/since.go`
- Create: `internal/source/since_test.go`

## 設計筆記

- `Source` 介面的 `Fetch` 回傳 **slice**（MVP 資料量不大）。若未來要改 streaming，可以加一個 `FetchStream` method，舊介面保留。
- `ParseSince` 接受的 duration 單位：`s`（秒）、`m`（分）、`h`（時）、`d`（日）、`w`（週）。`time.ParseDuration` 不支援 `d` / `w`，因此自己解析。
- `ParseSince` 以 `now` 作為參考點（測試時可注入），回傳 `now - duration` 或解析 RFC-3339 的絕對時間。
- 回傳 UTC。

## Steps

- [ ] **Step 1: 定義介面 `internal/source/source.go`**

```go
// Package source fetches mails from a mail provider.
// The Source interface decouples the pipeline from any specific protocol;
// MVP uses IMAPSource.
package source

import (
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

type Source interface {
	// Fetch returns all mails received at or after `since`.
	// Also reports the folder's current UIDVALIDITY — callers
	// use this to detect mailbox rebuilds.
	Fetch(folder string, since time.Time) (mails []mail.Mail, uidValidity uint32, err error)
}
```

- [ ] **Step 2: 寫 since parser 失敗測試**

`internal/source/since_test.go`：

```go
package source

import (
	"testing"
	"time"
)

func TestParseSinceRelative(t *testing.T) {
	ref := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
	}{
		{"30s", ref.Add(-30 * time.Second)},
		{"5m", ref.Add(-5 * time.Minute)},
		{"24h", ref.Add(-24 * time.Hour)},
		{"3d", ref.Add(-3 * 24 * time.Hour)},
		{"1w", ref.Add(-7 * 24 * time.Hour)},
	}
	for _, tt := range tests {
		got, err := ParseSince(tt.in, ref)
		if err != nil {
			t.Errorf("ParseSince(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseSince(%q): got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseSinceAbsoluteRFC3339(t *testing.T) {
	got, err := ParseSince("2026-04-01T00:00:00Z", time.Now())
	if err != nil {
		t.Fatalf("ParseSince: %v", err)
	}
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParseSinceInvalid(t *testing.T) {
	bad := []string{"", "abc", "3", "d3", "3x", "3.5d", "-3d"}
	for _, s := range bad {
		if _, err := ParseSince(s, time.Now()); err == nil {
			t.Errorf("ParseSince(%q): expected error, got nil", s)
		}
	}
}
```

- [ ] **Step 3: 跑測試確認失敗**

```bash
go test ./internal/source/...
```

預期：編譯錯誤。

- [ ] **Step 4: 實作 `internal/source/since.go`**

```go
package source

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseSince converts a user-supplied duration expression or RFC-3339
// timestamp to an absolute time. Relative expressions are subtracted
// from `ref` (normally time.Now()).
//
// Accepted relative units: s, m, h, d, w. Negative values are rejected.
func ParseSince(s string, ref time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("since: empty")
	}

	// Absolute RFC-3339 first (contains ':' or '-' after digit → likely a timestamp).
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	// Relative: <digits><unit>.
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("since: %q too short", s)
	}
	unit := s[len(s)-1]
	numPart := s[:len(s)-1]
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return time.Time{}, fmt.Errorf("since: %q is not <N><unit>", s)
	}
	if n < 0 {
		return time.Time{}, fmt.Errorf("since: negative value %q", s)
	}
	var d time.Duration
	switch unit {
	case 's':
		d = time.Duration(n) * time.Second
	case 'm':
		d = time.Duration(n) * time.Minute
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("since: unknown unit %q in %q", string(unit), s)
	}
	_ = strings.TrimSpace // placeholder to keep strings import if refactored
	return ref.Add(-d).UTC(), nil
}
```

> 備註：`strings.TrimSpace` 是殘留；如果編譯抱怨 `strings` 未使用，請直接把 import 拿掉。

- [ ] **Step 5: 跑測試確認通過**

```bash
go test ./internal/source/...
```

預期：PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/source
git commit -m "新增 Source 介面與 since 時間解析器"
```

## 驗收

- `go test ./internal/source/...` 全過，含 invalid case（負號、缺單位、亂碼）。
- RFC-3339 和相對時間都能解析。
- `Source` 介面就位，下一個 task 要實作它。
