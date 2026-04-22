# Task 08 — IMAPSource: 連線 + SEARCH SINCE

**目標**：實作 `IMAPSource` 的前半段 — 用 `emersion/go-imap/v2` 連上 TLS:993、登入、選 folder、跑 `SEARCH SINCE <date>` 拿到 UID 清單、讀 UIDVALIDITY。這個 task **還不抓整封信**，只取 UID 列表。

**依賴**：Task 07。

## 產出檔案

- Create: `internal/source/imap.go`
- Create: `internal/source/imap_test.go`（整合測試，預設 skip）

## 設計筆記

- `go-imap/v2` 目前 API 路徑是 `github.com/emersion/go-imap/v2` 搭配 `github.com/emersion/go-imap/v2/imapclient`。版本仍在演進，task 實作時以安裝後的實際 API 為準；以下程式碼是以 v2 常見 API 寫的 sketch，可能需要微調。
- 整合測試需要真實信箱，因此預設在 `testing.Short()` 或缺少 `MAIL_AGENT_IT=1` 環境變數時 skip。
- 所有錯誤都包在 `fmt.Errorf(... : %w, err)` 裡，保留鏈。

## Steps

- [ ] **Step 1: 加入 go-imap 依賴**

```bash
go get github.com/emersion/go-imap/v2
```

> 若 `go get` 出現 API 變更警告，查對應的最新 release notes。Task 實作時以實際可編譯的 API 為準。

- [ ] **Step 2: 實作 `internal/source/imap.go` 骨架**

```go
package source

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/austinjan/mail-agent/internal/mail"
)

type IMAPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

type IMAPSource struct {
	cfg IMAPConfig
}

func NewIMAPSource(cfg IMAPConfig) *IMAPSource {
	return &IMAPSource{cfg: cfg}
}

// compile-time assertion
var _ Source = (*IMAPSource)(nil)

func (s *IMAPSource) Fetch(folder string, since time.Time) ([]mail.Mail, uint32, error) {
	c, err := s.dial()
	if err != nil {
		return nil, 0, fmt.Errorf("imap dial: %w", err)
	}
	defer c.Close()

	if err := c.Login(s.cfg.User, s.cfg.Password).Wait(); err != nil {
		return nil, 0, fmt.Errorf("imap login: %w", err)
	}

	selectData, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("imap select %q: %w", folder, err)
	}
	uidValidity := selectData.UIDValidity

	// SEARCH SINCE <date> — IMAP SINCE only has day granularity.
	criteria := &imap.SearchCriteria{
		Since: since,
	}
	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("imap search: %w", err)
	}
	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, uidValidity, nil
	}

	// Full fetch is in the next task. For this task we return an
	// empty slice with uidValidity set and let Task 09 extend it.
	_ = uids
	return nil, uidValidity, nil
}

func (s *IMAPSource) dial() (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return imapclient.DialTLS(addr, &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: s.cfg.Host},
	})
}
```

> **API 檢查**：`selectData.UIDValidity`、`searchData.AllUIDs()` 是 v2 預期 API。若實際 binding 不同，請在此調整，並在 commit message 註明採用的 API 版本。

- [ ] **Step 3: 寫整合測試 `internal/source/imap_test.go`**

```go
package source

import (
	"os"
	"testing"
	"time"
)

// Integration test — needs a real IMAP account.
// Enable with `MAIL_AGENT_IT=1 go test ./internal/source/...`.
// Reads credentials from env to avoid committing secrets.
func TestIMAPSourceFetchLive(t *testing.T) {
	if os.Getenv("MAIL_AGENT_IT") != "1" {
		t.Skip("integration test; set MAIL_AGENT_IT=1 to run")
	}
	cfg := IMAPConfig{
		Host:     os.Getenv("MAIL_AGENT_IMAP_HOST"),
		Port:     993,
		User:     os.Getenv("MAIL_AGENT_IMAP_USER"),
		Password: os.Getenv("MAIL_AGENT_IMAP_PASS"),
	}
	if cfg.Host == "" || cfg.User == "" || cfg.Password == "" {
		t.Fatal("MAIL_AGENT_IMAP_HOST / USER / PASS must be set")
	}
	src := NewIMAPSource(cfg)
	_, uidValidity, err := src.Fetch("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if uidValidity == 0 {
		t.Error("expected non-zero UIDVALIDITY")
	}
}
```

- [ ] **Step 4: 確認 unit 測試跑得過（不觸發整合測試）**

```bash
go test ./internal/source/...
```

預期：`since_test.go` PASS；`TestIMAPSourceFetchLive` SKIP。

- [ ] **Step 5: 本地實際跑一次整合測試**

設一組可用的 Gmail App Password 或 testing 信箱：

```bash
MAIL_AGENT_IT=1 \
MAIL_AGENT_IMAP_HOST=imap.gmail.com \
MAIL_AGENT_IMAP_USER=you@gmail.com \
MAIL_AGENT_IMAP_PASS=app-password \
go test -run TestIMAPSourceFetchLive ./internal/source/...
```

預期：連線成功、`uidValidity > 0`、返回空 mail slice（完整抓取在下一個 task）。

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/source
git commit -m "IMAPSource 實作連線與 SEARCH SINCE"
```

## 驗收

- 單元測試不需要網路即可通過。
- 帶上 env 執行整合測試能拿到 `uidValidity > 0`。
- `IMAPSource` 滿足 `Source` 介面（compile-time assertion）。
