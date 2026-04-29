# Task 06: Store SaveAttachment 與 Content-Hashed 檔案

**目標**：實作 `SqliteStore.SaveAttachment`。附件內容先計算 `sha256`，再寫入 `<attachmentDir>/<ab>/<sha256>`，並在 `attachments` 資料表新增對應 row。相同內容的附件應共用同一個實體檔案。

**依賴**：Task 05 已完成。

## 產出檔案

- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_test.go`

## 設計筆記

- 實體檔案路徑採 content-hashed layout：`attachments/<前兩碼>/<sha256>`
- `file_path` 儲存相對路徑，例如 `b9/<sha256>`
- 寫檔使用 temp file + rename，避免留下半寫入檔案
- 相同 sha256 可共用一個實體檔案，但 `attachments` table 仍可有多筆 row

## Steps

- [x] **Step 1: 先寫失敗測試**

在 `internal/store/sqlite_test.go` 中新增：

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
		UIDValidity: 1,
		UID:         1,
		Folder:      "INBOX",
		ReceivedAt:  time.Now().UTC(),
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

	wantSha := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	wantPath := filepath.Join(attDir, "b9", wantSha)
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read stored file %q: %v", wantPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("stored content mismatch: got %q want %q", got, content)
	}

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

	m1, err := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 1, Folder: "INBOX", ReceivedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("SaveMail m1: %v", err)
	}
	m2, err := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 2, Folder: "INBOX", ReceivedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("SaveMail m2: %v", err)
	}

	a := mail.Attachment{Filename: "x.bin", Content: []byte("shared content")}
	if err := s.SaveAttachment(m1, a); err != nil {
		t.Fatalf("SaveAttachment m1: %v", err)
	}
	if err := s.SaveAttachment(m2, a); err != nil {
		t.Fatalf("SaveAttachment m2: %v", err)
	}

	fileCount := 0
	err = filepath.Walk(attDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk attachment dir: %v", err)
	}
	if fileCount != 1 {
		t.Errorf("expected 1 physical file, found %d", fileCount)
	}

	var rows int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM attachments`).Scan(&rows); err != nil {
		t.Fatalf("count attachment rows: %v", err)
	}
	if rows != 2 {
		t.Errorf("expected 2 attachment rows, got %d", rows)
	}
}
```

- [x] **Step 2: 跑測試確認先失敗**

```bash
go test ./internal/store/...
```

預期：`SaveAttachment` 尚未實作，新測試會失敗。

- [x] **Step 3: 實作 `SaveAttachment`**

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
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("write tmp: %w", err)
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("close tmp: %w", err)
		}
		if err := os.Rename(tmp.Name(), finalPath); err != nil {
			_ = os.Remove(tmp.Name())
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

- [x] **Step 4: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：兩個新測試都通過。

- [x] **Step 5: Commit**

```bash
git add internal/store
git commit -m "Implement content-hashed attachment persistence"
```

## 驗收

- 相同內容的附件只會有一份實體檔案
- `attachments.file_path` 會是相對路徑 `ab/<sha256>`
- 寫檔採用 temp file + rename，降低半寫入風險
