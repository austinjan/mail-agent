package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/austinjan/mail-agent/internal/config"
	"github.com/austinjan/mail-agent/internal/core"
	"github.com/austinjan/mail-agent/internal/extract"
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
	case "extract":
		runExtract(os.Args[2:])
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
  mail-agent extract enqueue --since=<duration> [--config=./config.yaml]
  mail-agent extract run [--limit=20] [--config=./config.yaml]
  mail-agent extract show --mail-id=<id> [--config=./config.yaml]
  mail-agent version

Examples:
  mail-agent read --since=3d
  mail-agent read --since=24h --folder=INBOX
  mail-agent read --since=2026-04-01T00:00:00Z --config=./config.yaml
  mail-agent extract enqueue --since=24h
  mail-agent extract run --limit=20
  mail-agent extract show --mail-id=1
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

func runExtract(args []string) {
	if len(args) < 1 {
		extractUsage()
		return
	}

	switch args[0] {
	case "enqueue":
		runExtractEnqueue(args[1:])
	case "run":
		runExtractRun(args[1:])
	case "show":
		runExtractShow(args[1:])
	case "-h", "--help", "help":
		extractUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown extract command: %s\n", args[0])
		extractUsage()
	}
}

func extractUsage() {
	fmt.Fprint(os.Stderr, `mail-agent extract processes stored mails and attachments.

Usage:
  mail-agent extract enqueue --since=<duration> [--config=./config.yaml]
  mail-agent extract run [--limit=20] [--config=./config.yaml]
  mail-agent extract show --mail-id=<id> [--config=./config.yaml]
`)
}

func runExtractEnqueue(args []string) {
	fs := flag.NewFlagSet("extract enqueue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sinceStr := fs.String("since", "", "required: 3d | 1w | 24h | RFC-3339 timestamp")
	cfgPath := fs.String("config", "config.yaml", "path to YAML config")
	if err := fs.Parse(args); err != nil {
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, st, ok := openConfiguredStore(*cfgPath, logger)
	if !ok {
		return
	}
	defer st.Close()

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

	stats, err := st.EnqueueExtractionJobs(since)
	if err != nil {
		logger.Error("", "event", "extract_enqueue_failed", "error", err.Error())
		return
	}
	logger.Info("", "event", "extract_enqueue_done", "body_jobs", stats.BodyJobs, "attachment_jobs", stats.AttachmentJobs)
}

func runExtractRun(args []string) {
	fs := flag.NewFlagSet("extract run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 20, "maximum pending jobs to process")
	cfgPath := fs.String("config", "config.yaml", "path to YAML config")
	if err := fs.Parse(args); err != nil {
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	_, st, ok := openConfiguredStore(*cfgPath, logger)
	if !ok {
		return
	}
	defer st.Close()

	p := extract.New(st, logger)
	if _, err := p.Run(*limit); err != nil {
		logger.Error("", "event", "extract_run_failed", "error", err.Error())
	}
}

func runExtractShow(args []string) {
	fs := flag.NewFlagSet("extract show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mailIDStr := fs.String("mail-id", "", "mail id to show")
	cfgPath := fs.String("config", "config.yaml", "path to YAML config")
	if err := fs.Parse(args); err != nil {
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	_, st, ok := openConfiguredStore(*cfgPath, logger)
	if !ok {
		return
	}
	defer st.Close()

	if *mailIDStr == "" {
		logger.Error("", "event", "mail_id_missing", "hint", "pass --mail-id=<id>")
		return
	}
	mailID, err := strconv.ParseInt(*mailIDStr, 10, 64)
	if err != nil || mailID <= 0 {
		logger.Error("", "event", "mail_id_invalid", "input", *mailIDStr)
		return
	}

	fields, err := st.ExtractedFieldsByMailID(mailID)
	if err != nil {
		logger.Error("", "event", "extract_show_failed", "error", err.Error())
		return
	}
	for _, f := range fields {
		attrs := []any{
			"event", "extracted_field",
			"mail_id", f.MailID,
			"job_id", f.JobID,
			"field_name", f.FieldName,
			"field_value", f.FieldValue,
			"unit", f.Unit,
			"confidence", f.Confidence,
			"evidence", f.EvidenceText,
			"source_type", f.SourceType,
			"source_label", f.SourceLabel,
		}
		if f.AttachmentID != nil {
			attrs = append(attrs, "attachment_id", *f.AttachmentID)
		}
		logger.Info("", attrs...)
	}
	logger.Info("", "event", "extract_show_done", "mail_id", mailID, "fields", len(fields))
}

func openConfiguredStore(cfgPath string, logger *slog.Logger) (*config.Config, *store.SqliteStore, bool) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("", "event", "config_load_failed", "error", err.Error())
		return nil, nil, false
	}
	st, err := store.OpenSQLite(cfg.Database.Path, cfg.Attachments.Dir)
	if err != nil {
		logger.Error("", "event", "store_open_failed", "error", err.Error())
		return nil, nil, false
	}
	return cfg, st, true
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
