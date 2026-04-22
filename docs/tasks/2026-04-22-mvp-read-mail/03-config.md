# Task 03 — Config 載入

**目標**：用 `gopkg.in/yaml.v3` 解析 `config.yaml`，產出 strongly-typed 結構。附 `config.example.yaml`。

**依賴**：Task 01。

## 產出檔案

- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/testdata/valid.yaml`
- Create: `config.example.yaml`

## Steps

- [ ] **Step 1: 加入 yaml 依賴**

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 2: 準備測試 fixture**

`internal/config/testdata/valid.yaml`：

```yaml
imap:
  host: imap.gmail.com
  port: 993
  user: austin.jan@gmail.com
  password: testpass
  folder: INBOX
defaults:
  since: 24h
database:
  path: ./mail-agent.db
attachments:
  dir: ./attachments
```

- [ ] **Step 3: 寫失敗測試**

`internal/config/config_test.go`：

```go
package config

import "testing"

func TestLoadValid(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IMAP.Host != "imap.gmail.com" {
		t.Errorf("IMAP.Host: got %q", cfg.IMAP.Host)
	}
	if cfg.IMAP.Port != 993 {
		t.Errorf("IMAP.Port: got %d", cfg.IMAP.Port)
	}
	if cfg.IMAP.User != "austin.jan@gmail.com" {
		t.Errorf("IMAP.User: got %q", cfg.IMAP.User)
	}
	if cfg.IMAP.Password != "testpass" {
		t.Errorf("IMAP.Password: got %q", cfg.IMAP.Password)
	}
	if cfg.IMAP.Folder != "INBOX" {
		t.Errorf("IMAP.Folder: got %q", cfg.IMAP.Folder)
	}
	if cfg.Defaults.Since != "24h" {
		t.Errorf("Defaults.Since: got %q", cfg.Defaults.Since)
	}
	if cfg.Database.Path != "./mail-agent.db" {
		t.Errorf("Database.Path: got %q", cfg.Database.Path)
	}
	if cfg.Attachments.Dir != "./attachments" {
		t.Errorf("Attachments.Dir: got %q", cfg.Attachments.Dir)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
```

- [ ] **Step 4: 跑測試確認失敗**

```bash
go test ./internal/config/...
```

預期：編譯錯誤（`Load` / `Config` 未定義）。

- [ ] **Step 5: 實作 `internal/config/config.go`**

```go
// Package config loads mail-agent's YAML config file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type IMAPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Folder   string `yaml:"folder"`
}

type DefaultsConfig struct {
	Since string `yaml:"since"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AttachmentsConfig struct {
	Dir string `yaml:"dir"`
}

type Config struct {
	IMAP        IMAPConfig        `yaml:"imap"`
	Defaults    DefaultsConfig    `yaml:"defaults"`
	Database    DatabaseConfig    `yaml:"database"`
	Attachments AttachmentsConfig `yaml:"attachments"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	return &cfg, nil
}
```

- [ ] **Step 6: 跑測試確認通過**

```bash
go test ./internal/config/...
```

預期：PASS。

- [ ] **Step 7: 產出 `config.example.yaml`**

```yaml
# config.example.yaml — copy to config.yaml and fill in your secrets
imap:
  host: imap.gmail.com
  port: 993
  user: your-email@gmail.com
  password: xxxxxxxxxxxxxxxx   # Gmail App Password, not your login password
  folder: INBOX
defaults:
  since: 24h
database:
  path: ./mail-agent.db
attachments:
  dir: ./attachments
```

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/config config.example.yaml
git commit -m "新增 YAML config 載入器"
```

## 驗收

- `go test ./internal/config/...` 全過。
- `config.example.yaml` 存在；`config.yaml` 仍在 `.gitignore` 內。
- Password 欄位以明碼放 YAML（這是 design D9 的決定，MVP 不做 keychain / env var）。
