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
