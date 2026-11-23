package cmd

var initExplanation = explanation{
	Name:     "init",
	Synopsis: "Initialize nextdeploy.yml for an existing Next.js app or scaffold a new fullstack starter.",
	Summary: "`init` inspects the current directory for Next.js. When absent, it scaffolds a " +
		"complete fullstack starter for Cloudflare Workers (D1 or BYO Postgres via Hyperdrive), " +
		"VPS, or AWS. For an existing Next.js app targeting Cloudflare, it offers either " +
		"infra-only scaffolding (--mode=scaffold) or read-only infra sniffing (--mode=sniff) " +
		"from package.json, wrangler.*, and source files. Existing files are never overwritten; " +
		"sniff writes nextdeploy.suggested.yml when nextdeploy.yml already exists.",
	Phases: []phase{
		{
			Num:       1,
			Title:     "Inspect project directory",
			Narrative: "Reads package.json (Next.js version + slugified app name) and package manager. Routes empty or non-Next.js directories to fullstack starter scaffolding.",
			Ref:       "cli/internal/initialcommand/init.go",
			Output:    "existing Next.js vs new starter path",
		},
		{
			Num:       2,
			Title:     "Select target & mode",
			Narrative: "Resolves --target (vps|aws|cloudflare), --mode (scaffold|sniff for Cloudflare), and --db (d1|byo|none for Cloudflare scaffold) from flags or interactive prompts.",
			Ref:       "cli/internal/initialcommand/init.go",
			Output:    "target, mode, and DB variant",
		},
		{
			Num:       3,
			Title:     "Scaffold or sniff & write config",
			Narrative: "Cloudflare scaffold writes nextdeploy.yml, middleware.ts (or proxy.ts on Next 16+), lib/env.ts, migrations, and CI without overwriting existing files. Cloudflare sniff inspects wrangler.* + source code and writes nextdeploy.yml (0600) or nextdeploy.suggested.yml if one already exists.",
			Ref:       "cli/internal/scaffold/scaffold.go",
			Output:    "nextdeploy.yml (and optional starter/infra files)",
		},
	},
}

func init() {
	registerExplain(initCmd, &initExplanation)
}
