package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"github.com/austinjan/mail-agent/internal/config"
	"github.com/austinjan/mail-agent/internal/core"
	"github.com/austinjan/mail-agent/internal/source"
	"github.com/austinjan/mail-agent/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "read":
		runRead(os.Args[2:])
	case "version":
		fmt.Println(versionString())
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mail-agent fetches mails from IMAP into local storage.

Usage:
  mail-agent read --since=<duration> [--folder=INBOX] [--config=./config.yaml]
  mail-agent version

Examples:
  mail-agent read --since=3d
  mail-agent read --since=24h --folder=INBOX
  mail-agent read --since=2026-04-01T00:00:00Z --config=./config.yaml
`)
}

func runRead(args []string) {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sinceStr := fs.String("since", "", "required: 3d | 1w | 24h | RFC-3339 timestamp")
	folder := fs.String("folder", "", "IMAP folder; overrides config")
	cfgPath := fs.String("config", "config.yaml", "path to YAML config")
	if err := fs.Parse(args); err != nil {
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("", "event", "config_load_failed", "error", err.Error())
		return
	}

	effectiveSince := *sinceStr
	if effectiveSince == "" {
		effectiveSince = cfg.Defaults.Since
	}
	if effectiveSince == "" {
		logger.Error("", "event", "since_missing", "hint", "pass --since=... or set defaults.since in config")
		return
	}
	since, err := source.ParseSince(effectiveSince, time.Now().UTC())
	if err != nil {
		logger.Error("", "event", "since_parse_failed", "input", effectiveSince, "error", err.Error())
		return
	}

	effectiveFolder := *folder
	if effectiveFolder == "" {
		effectiveFolder = cfg.IMAP.Folder
	}
	if effectiveFolder == "" {
		effectiveFolder = "INBOX"
	}

	imapPort := cfg.IMAP.Port
	if imapPort == 0 {
		imapPort = 993
	}

	st, err := store.OpenSQLite(cfg.Database.Path, cfg.Attachments.Dir)
	if err != nil {
		logger.Error("", "event", "store_open_failed", "error", err.Error())
		return
	}
	defer st.Close()

	src := source.NewIMAPSource(source.IMAPConfig{
		Host:     cfg.IMAP.Host,
		Port:     imapPort,
		User:     cfg.IMAP.User,
		Password: cfg.IMAP.Password,
	})

	p := core.New(src, st, logger)
	if _, err := p.Run(effectiveFolder, since); err != nil {
		logger.Error("", "event", "pipeline_error", "error", err.Error())
	}
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
