# Task 06 — Store: SaveAttachment + content-hashed 檔案

**目標**：實作 `SqliteStore.SaveAttachment`。邏輯：計算 sha256，算出目標路徑 `<attachmentDir>/<ab>/<sha256>`，若檔案不存在則寫入（存在就跳過 — 相同內容共用一個實體），最後在 `attachments` 資料表插入 row。

**依賴**：Task 05。

## 產出檔案

- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_test.go`

## 設計筆記

- 寫檔用**先寫暫存檔 → rename** 的 atomic pattern，避免 crash 時留下半寫的檔案。
- `file_path` 存**相對於 `attachmentDir` 的路徑**（`ab/abcdef...`），不存絕對路徑 — 這樣整個工作目錄搬家仍可用。
- 同一封信的同一個附件（相同 sha256）若 attachment row 已存在，仍寫新 row（同一個 mail 可能有兩個完全一樣的附件）。MVP 不去重 row，只去重實體檔案。

## Steps

- [ ] **Step 1: 寫失敗測試**

追加到 `internal/store/sqlite_test.go`：

```go
func TestSaveAttachmentWritesFile(t *testing.T) {
	dir := t.TempDir()
	attDir := filepath.Join(dir, "attachments")
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	mailID, err := s.SaveMail(mail.Mail{
		UIDValidity: 1, UID: 1, Folder: "INBOX",
		ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}

	content := []byte("hello world")
	a := mail.Attachment{
		Filename:    "greeting.txt",
		ContentType: "text/plain",
		Content:     content,
	}
	if err := s.SaveAttachment(mailID, a); err != nil {
		t.Fatalf("SaveAttachment: %v", err)
	}

	// sha256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
	wantSha := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	wantPath := filepath.Join(attDir, "b9", wantSha)
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read stored file %q: %v", wantPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("stored content mismatch: got %q want %q", got, content)
	}

	// Row exists in attachments table with correct sha256 and relative path.
	var sha, relPath string
	var size int64
	err = s.db.QueryRow(
		`SELECT sha256, size_bytes, file_path FROM attachments WHERE mail_id = ?`,
		mailID,
	).Scan(&sha, &size, &relPath)
	if err != nil {
		t.Fatalf("query attachments: %v", err)
	}
	if sha != wantSha {
		t.Errorf("sha256: got %q want %q", sha, wantSha)
	}
	if size != int64(len(content)) {
		t.Errorf("size_bytes: got %d want %d", size, len(content))
	}
	if relPath != "b9/"+wantSha {
		t.Errorf("file_path: got %q want %q", relPath, "b9/"+wantSha)
	}
}

func TestSaveAttachmentDeduplicatesFile(t *testing.T) {
	dir := t.TempDir()
	attDir := filepath.Join(dir, "attachments")
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	// Two different mails, same attachment content.
	m1, _ := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 1, Folder: "INBOX", ReceivedAt: time.Now().UTC()})
	m2, _ := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 2, Folder: "INBOX", ReceivedAt: time.Now().UTC()})

	a := mail.Attachment{Filename: "x.bin", Content: []byte("shared content")}
	if err := s.SaveAttachment(m1, a); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAttachment(m2, a); err != nil {
		t.Fatal(err)
	}

	// Only one physical file exists under attDir.
	var fileCount int
	filepath.Walk(attDir, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	if fileCount != 1 {
		t.Errorf("expected 1 physical file, found %d", fileCount)
	}

	// But two rows in attachments table.
	var rows int
	s.db.QueryRow(`SELECT COUNT(*) FROM attachments`).Scan(&rows)
	if rows != 2 {
		t.Errorf("expected 2 attachment rows, got %d", rows)
	}
}
```

test file 補上 import：`"bytes"`, `"crypto/sha256"`（給 test 對照用；若用 constant literal 就不需）、`"os"`, `"path/filepath"`。

- [ ] **Step 2: 跑測試確認失敗**

```bash
go test ./internal/store/...
```

預期：`SaveAttachment: not implemented`，兩個新測試 fail。

- [ ] **Step 3: 實作 `SaveAttachment`**

替換 `SaveAttachment` stub：

```go
func (s *SqliteStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	sum := sha256.Sum256(a.Content)
	sumHex := hex.EncodeToString(sum[:])
	prefix := sumHex[:2]
	relPath := filepath.ToSlash(filepath.Join(prefix, sumHex))

	prefixDir := filepath.Join(s.attachmentDir, prefix)
	if err := os.MkdirAll(prefixDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", prefixDir, err)
	}

	finalPath := filepath.Join(prefixDir, sumHex)
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		tmp, err := os.CreateTemp(prefixDir, "att-*.tmp")
		if err != nil {
			return fmt.Errorf("create tmp: %w", err)
		}
		if _, err := tmp.Write(a.Content); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return fmt.Errorf("write tmp: %w", err)
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return fmt.Errorf("close tmp: %w", err)
		}
		if err := os.Rename(tmp.Name(), finalPath); err != nil {
			os.Remove(tmp.Name())
			return fmt.Errorf("rename tmp: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat %q: %w", finalPath, err)
	}

	_, err := s.db.Exec(
		`INSERT INTO attachments (mail_id, filename, content_type, size_bytes, sha256, file_path)
		 VALUES (?,?,?,?,?,?)`,
		mailID, a.Filename, a.ContentType, len(a.Content), sumHex, relPath,
	)
	if err != nil {
		return fmt.Errorf("insert attachment row: %w", err)
	}
	return nil
}
```

補 import：`"crypto/sha256"`, `"encoding/hex"`, `"os"`, `"path/filepath"`。

- [ ] **Step 4: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：PASS，含兩個新測試。

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "SqliteStore 支援 content-hashed 附件儲存"
```

## 驗收

- 相同內容的附件在磁碟上只有一份實體檔案。
- `attachments.file_path` 是相對路徑（`ab/<sha256>`）。
- 寫入過程中斷不會留下半寫檔案（tmp + rename pattern）。
