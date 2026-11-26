package cmd

const logsGoFile = "cli/cmd/logs.go"

var logsExplanation = explanation{
	Name:     "logs",
	Synopsis: "Stream, query, and triage application logs with fault attribution.",
	Summary: "`logs` tails live logs, queries historical Cloudflare Workers Observability " +
		"telemetry (`--since`), or streams exported JSON/NDJSON log files in constant memory " +
		"(`logs analyze <file>` or `logs -f <file>`). Multi-fragment errors are correlated by " +
		"`requestId` and labeled `[APP]`, `[NEXTDEPLOY]`, or `[PLATFORM]` with concrete fix hints.",
	Phases: []phase{
		{
			Num:       1,
			Title:     "Resolve mode (offline analyze vs live/historical)",
			Narrative: "If --file or `logs analyze <file>` is used, NextDeploy streams the JSON/NDJSON export directly without needing nextdeploy.yml. Otherwise config.Load resolves VPS vs Serverless target.",
			Ref:       logsGoFile,
			Function:  "runAnalyzeFile",
		},
		{
			Num:       2,
			Title:     "Connect or stream telemetry",
			Narrative: "Cloudflare live mode uses the native Go WebSocket Tail API (with wrangler fallback); --since queries Workers Observability Telemetry API; VPS tails systemd/daemon logs via SSH; AWS tails CloudWatch.",
			Ref:       "cli/internal/serverless/cloudflare/tail.go",
			Function:  "CloudflareProvider.TailLogsWithOptions",
		},
		{
			Num:       3,
			Title:     "Correlate, deduplicate & classify fault owner",
			Narrative: "Streaming events are merged by requestId, redundant stack fragments are collapsed, and each failure is classified as APP, NEXTDEPLOY, or PLATFORM with an actionable hint.",
			Ref:       "cli/internal/logs/classify.go",
			Function:  "Classify",
		},
		{
			Num:       4,
			Title:     "Render concise summary or NDJSON export",
			Narrative: "Outputs grouped triage tables (--since / analyze), clean single-cause tail lines, or deduplicated NDJSON (--out) for grep/jq/DuckDB.",
			Ref:       "cli/internal/logs/stream_analyzer.go",
			Function:  "FormatReport",
		},
	},
}

func init() {
	registerExplain(logsCmd, &logsExplanation)
}
