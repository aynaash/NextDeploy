package cmd

var prepareExplanation = explanation{
	Name:     "prepare",
	Synopsis: "Provision a VPS with the tools NextDeploy's daemon needs.",
	Summary: "`prepare` provisions the declared VPS server(s) with the JS runtimes, " +
		"Caddy support directories, log rotation, fail2ban jails and the " +
		"nextdeployd control plane. By default it runs in AGENT mode: the " +
		"nextdeployd static binary is installed on the server and does the work " +
		"there, so the target needs no Python and this machine needs no Ansible. " +
		"Idempotent — every step checks before it changes, so re-running on a " +
		"prepared server is a fast no-op. `--ansible` selects the legacy playbook.",
	Phases: []phase{
		{
			Num:       1,
			Title:     "Context + signal handling",
			Narrative: "Sets a timeout for the whole run (provisioning over slow SSH can take minutes) and installs a SIGINT/SIGTERM trap so Ctrl-C cancels cleanly.",
			Ref:       "cli/cmd/prepare.go:70",
			Output:    "context.Context with cancellation",
		},
		{
			Num:       2,
			Title:     "Resolve target server",
			Narrative: "Reads nextdeploy.yml's servers[] list and refuses a root SSH login unless --allow-root is passed.",
			Ref:       "cli/cmd/prepare.go:83",
			Function:  "resolveTargetServer",
			Output:    "serverName, ServerConfig",
		},
		{
			Num:       3,
			Title:     "Bootstrap the agent (default path)",
			Narrative: "Runs a POSIX-sh script over SSH that detects the target's architecture, resolves the latest release tag, downloads nextdeployd-linux-<arch>, verifies its ELF magic (a bad URL returns an HTML 404 page, not a binary) and installs it to /usr/local/bin.",
			Ref:       "cli/cmd/prepare_agent.go:38",
			Function:  "bootstrapScript + runPrepareAgent",
			Notes: []string{
				"Needs only a shell and curl (or wget) on the target — no Python interpreter.",
				"Handles hosts with no sudo binary by checking id -u first.",
			},
		},
		{
			Num:       4,
			Title:     "Run nextdeployd prepare on the target",
			Narrative: "The daemon provisions itself: package index, base packages, the nextdeploy user/group, /opt/nextdeploy tree, Node (NodeSource signed repo), Corepack pnpm/yarn shims, Bun, Caddy log dirs, logrotate and the fail2ban jails. Each step declares a Done predicate; satisfied steps are skipped.",
			Ref:       "daemon/internal/prepare/plan.go:90",
			Function:  "prepare.Plan + prepare.RunSteps",
			Output:    "a per-step report; non-zero exit if a required step failed",
			Notes: []string{
				"A required failure stops the run — later steps assume earlier ones landed.",
				"Optional steps (Bun, fail2ban) warn and continue.",
				"Every package manager the daemon can emit an ExecStart for is installed, so a pnpm/yarn app can't ship successfully and then fail to start.",
			},
		},
		{
			Num:       5,
			Title:     "Legacy: ansible-playbook (--ansible only)",
			Narrative: "Ensures ansible-playbook locally, writes a temp inventory + the embedded playbook, and runs it. Requires python3 on the target because Ansible executes its modules there.",
			Ref:       "cli/cmd/prepare.go:118",
			Function:  "ensureAnsible + writeInventory + runAnsible",
			Notes:     []string{"Kept until the agent path has proven parity in the field; the inventory sets ansible_python_interpreter=auto_silent so discovery doesn't assume a hard-coded path."},
		},
	},
}

func init() {
	registerExplain(prepareCmd, &prepareExplanation)
}
