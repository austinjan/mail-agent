package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/austinjan/mail-agent/internal/config"
	"github.com/austinjan/mail-agent/internal/core"
	"github.com/austinjan/mail-agent/internal/extract"
	"github.com/austinjan/mail-agent/internal/llm"
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
  mail-agent extract run [--limit=20] [--mode=llm|rules] [--config=./config.yaml]
  mail-agent extract show --mail-id=<id> [--config=./config.yaml]
  mail-agent extract export --out=extracted_fields.csv [--mail-id=<id>] [--config=./config.yaml]
  mail-agent version

Examples:
  mail-agent read --since=3d
  mail-agent read --since=24h --folder=INBOX
  mail-agent read --since=2026-04-01T00:00:00Z --config=./config.yaml
  mail-agent extract enqueue --since=24h
  mail-agent extract run --limit=20
  mail-agent extract run --mode=rules --limit=20
  mail-agent extract show --mail-id=1
  mail-agent extract export --out=extracted_fields.csv
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
	case "export":
		runExtractExport(args[1:])
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
  mail-agent extract run [--limit=20] [--mode=llm|rules] [--config=./config.yaml]
  mail-agent extract show --mail-id=<id> [--config=./config.yaml]
  mail-agent extract export --out=extracted_fields.csv [--mail-id=<id>] [--config=./config.yaml]
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
	mode := fs.String("mode", "llm", "extractor mode: llm or rules")
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

	extractor, ok := configuredExtractor(*mode, cfg, logger)
	if !ok {
		return
	}
	p := extract.NewWithExtractor(st, logger, extractor)
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

func runExtractExport(args []string) {
	fs := flag.NewFlagSet("extract export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outPath := fs.String("out", "extracted_fields.csv", "CSV output path")
	mailIDStr := fs.String("mail-id", "", "optional mail id filter")
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

	var mailID *int64
	if *mailIDStr != "" {
		id, err := strconv.ParseInt(*mailIDStr, 10, 64)
		if err != nil || id <= 0 {
			logger.Error("", "event", "mail_id_invalid", "input", *mailIDStr)
			return
		}
		mailID = &id
	}

	fields, err := st.ExtractedFields(mailID)
	if err != nil {
		logger.Error("", "event", "extract_export_failed", "error", err.Error())
		return
	}
	if err := writeExtractedFieldsCSV(*outPath, fields); err != nil {
		logger.Error("", "event", "extract_export_failed", "error", err.Error())
		return
	}
	logger.Info("", "event", "extract_export_done", "out", *outPath, "fields", len(fields))
}

func writeExtractedFieldsCSV(path string, fields []store.ExtractedField) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv %q: %w", path, err)
	}
	defer f.Close()

	// UTF-8 BOM keeps Traditional Chinese readable when opened directly in Excel.
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write csv bom: %w", err)
	}

	w := csv.NewWriter(f)
	if err := w.Write(typeSearchColumns()); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, row := range typeSearchRows(fields) {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func typeSearchColumns() []string {
	return []string{
		"Item",
		"CMH",
		"m",
		"RPM",
		"黏度",
		"比重",
		"SSVP管長",
		"機殼鑄造方式",
		"機型",
		"建議馬力",
		"額定馬力",
		"最大馬力",
		"RPM_2",
		"EFF",
		"直徑",
		"最大直徑",
		"流量%",
		"葉片角度",
		"備註",
	}
}

func typeSearchRows(fields []store.ExtractedField) [][]string {
	rowMap := map[string]map[string]string{}
	var order []string
	for _, field := range fields {
		item, name := splitItemField(field.FieldName)
		if item == "" {
			item = strconv.FormatInt(field.MailID, 10)
		}
		if _, ok := rowMap[item]; !ok {
			rowMap[item] = map[string]string{"Item": item}
			order = append(order, item)
		}
		name = typeSearchColumnName(name)
		rowMap[item][name] = field.FieldValue
		if field.EvidenceText != "" {
			appendNote(rowMap[item], field.EvidenceText)
		}
	}

	columns := typeSearchColumns()
	rows := make([][]string, 0, len(order))
	for idx, item := range order {
		rowMap[item]["Item"] = strconv.Itoa(idx + 1)
		row := make([]string, len(columns))
		for i, column := range columns {
			row[i] = rowMap[item][column]
		}
		rows = append(rows, row)
	}
	return rows
}

func typeSearchColumnName(name string) string {
	switch name {
	case "流量":
		return "CMH"
	case "揚程":
		return "m"
	default:
		return name
	}
}

func splitItemField(fieldName string) (string, string) {
	item, name, ok := strings.Cut(fieldName, ".")
	if !ok {
		return "", fieldName
	}
	if item == "" || name == "" {
		return "", fieldName
	}
	return item, name
}

func appendNote(row map[string]string, evidence string) {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" || strings.Contains(row["備註"], evidence) {
		return
	}
	if row["備註"] == "" {
		row["備註"] = evidence
		return
	}
	row["備註"] += " | " + evidence
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

func configuredExtractor(mode string, cfg *config.Config, logger *slog.Logger) (extract.Extractor, bool) {
	switch mode {
	case "rules":
		logger.Info("", "event", "extract_mode", "mode", "rules")
		return extract.RuleExtractor{}, true
	case "llm":
		provider := cfg.LLM.Provider
		if provider == "" {
			provider = "gemini"
		}
		model := cfg.LLM.Model
		apiKeyEnv := cfg.LLM.APIKeyEnv
		apiKey, usedEnv := llmAPIKey(provider, apiKeyEnv)
		if apiKey == "" {
			logger.Error("", "event", "llm_api_key_missing", "provider", provider, "env", apiKeyEnv, "fallback_envs", "Gkey,OPENAI_API_KEY", "hint", "set the environment variable or run extract with --mode=rules")
			return nil, false
		}
		switch provider {
		case "gemini":
			if model == "" {
				model = "gemini-2.5-flash"
			}
			logger.Info("", "event", "extract_mode", "mode", "llm", "provider", provider, "model", model, "api_key_env", usedEnv)
			return extract.NewLLMExtractor(llm.NewGeminiClient(apiKey, model)), true
		case "openai":
			if model == "" {
				model = "gpt-5-mini"
			}
			logger.Info("", "event", "extract_mode", "mode", "llm", "provider", provider, "model", model, "api_key_env", usedEnv)
			return extract.NewLLMExtractor(llm.NewClient(apiKey, model)), true
		default:
			logger.Error("", "event", "llm_provider_unsupported", "provider", provider)
			return nil, false
		}
	default:
		logger.Error("", "event", "extract_mode_invalid", "mode", mode, "hint", "use --mode=llm or --mode=rules")
		return nil, false
	}
}

func llmAPIKey(provider, primaryEnv string) (string, string) {
	envs := []string{primaryEnv}
	switch provider {
	case "gemini":
		envs = append(envs, "Gkey", "GEMINI_API_KEY", "OPENAI_API_KEY")
	case "openai":
		envs = append(envs, "OPENAI_API_KEY", "Gkey")
	default:
		envs = append(envs, "Gkey", "OPENAI_API_KEY")
	}
	for _, name := range envs {
		if name == "" {
			continue
		}
		if value := os.Getenv(name); value != "" {
			return value, name
		}
	}
	return "", ""
}

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
