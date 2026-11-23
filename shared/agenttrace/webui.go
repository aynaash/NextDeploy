package agenttrace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// StartTraceUI launches the embedded local dashboard on the specified port.
func StartTraceUI(projectDir string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(dashboardHTML))
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cp, _ := ReadLatestCheckpoint(projectDir)
		events, _ := ReadEvents(projectDir)
		modFiles, _ := GetGitModifiedFiles(projectDir)
		branch, _ := GetGitBranch(projectDir)

		payload := map[string]interface{}{
			"checkpoint":     cp,
			"events":         events,
			"modified_files": modFiles,
			"branch":         branch,
			"trace_dir":      GetTraceDir(projectDir),
		}
		_ = json.NewEncoder(w).Encode(payload)
	})

	mux.HandleFunc("/api/diff", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		cmd := exec.Command("git", "diff")
		cmd.Dir = projectDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get git diff: %v", err), http.StatusInternalServerError)
			return
		}
		if len(out) == 0 {
			cmdUntracked := exec.Command("git", "status", "--short")
			cmdUntracked.Dir = projectDir
			out, _ = cmdUntracked.CombinedOutput()
		}
		_, _ = w.Write(out)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server.ListenAndServe()
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>NextDeploy — Agent Trace Dashboard</title>
  <style>
    :root {
      --bg: #0b0f19;
      --card-bg: #111827;
      --border: #1f2937;
      --text: #f9fafb;
      --text-muted: #9ca3af;
      --primary: #3b82f6;
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      line-height: 1.5;
      padding: 24px;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding-bottom: 20px;
      border-bottom: 1px solid var(--border);
      margin-bottom: 24px;
    }
    .logo { display: flex; align-items: center; gap: 10px; font-weight: 700; font-size: 20px; }
    .badge {
      display: inline-block;
      padding: 4px 10px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .badge-primary { background: rgba(59, 130, 246, 0.15); color: var(--primary); border: 1px solid rgba(59, 130, 246, 0.3); }
    .badge-success { background: rgba(16, 185, 129, 0.15); color: var(--success); border: 1px solid rgba(16, 185, 129, 0.3); }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 24px; }
    @media (max-width: 768px) { .grid { grid-template-columns: 1fr; } }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 20px;
    }
    .card-title {
      font-size: 14px;
      text-transform: uppercase;
      letter-spacing: 1px;
      color: var(--text-muted);
      margin-bottom: 12px;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .field { margin-bottom: 10px; }
    .field-label { font-size: 12px; color: var(--text-muted); }
    .field-val { font-size: 15px; font-weight: 500; }
    .code-box {
      background: #06090e;
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 12px;
      font-family: var(--font-mono);
      font-size: 13px;
      color: #e2e8f0;
      overflow-x: auto;
      max-height: 250px;
      white-space: pre;
    }
    .btn {
      background: var(--primary);
      color: white;
      border: none;
      border-radius: 6px;
      padding: 8px 14px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }
    .btn:hover { opacity: 0.9; }
    .btn-secondary {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--text);
    }
    .btn-secondary:hover { background: #1f2937; }
    .timeline { display: flex; flex-direction: column; gap: 12px; }
    .event-item {
      padding: 12px;
      border-radius: 8px;
      background: #0d131f;
      border-left: 3px solid var(--primary);
      font-size: 13px;
    }
    .event-header { display: flex; justify-content: space-between; margin-bottom: 4px; font-weight: 600; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="logo">
        <span>🚀 NextDeploy</span>
        <span class="badge badge-primary">Agent Trace</span>
      </div>
      <div>
        <button class="btn btn-secondary" onclick="copyTakeover()">📋 Copy Takeover Brief</button>
      </div>
    </header>

    <div class="grid">
      <div class="card">
        <div class="card-title">
          <span>Active Agent Session</span>
          <span class="badge badge-success" id="agent-badge">Active</span>
        </div>
        <div class="field">
          <div class="field-label">Objective / Intent</div>
          <div class="field-val" id="session-objective">Loading intent...</div>
        </div>
        <div class="field">
          <div class="field-label">Agent & Model</div>
          <div class="field-val" id="session-model">—</div>
        </div>
        <div class="field">
          <div class="field-label">Session ID</div>
          <div class="field-val" id="session-id" style="font-family: var(--font-mono); font-size: 13px;">—</div>
        </div>
      </div>

      <div class="card">
        <div class="card-title">
          <span>Infrastructure & Deployment</span>
          <span class="badge badge-primary" id="deploy-target-badge">NextDeploy</span>
        </div>
        <div class="field">
          <div class="field-label">Target & Environment</div>
          <div class="field-val" id="deploy-target">—</div>
        </div>
        <div class="field">
          <div class="field-label">Live Preview / Production URL</div>
          <div class="field-val" id="deploy-url">Not yet deployed</div>
        </div>
        <div class="field">
          <div class="field-label">SSR Probing Status</div>
          <div class="field-val" id="deploy-health">Awaiting cutover</div>
        </div>
      </div>
    </div>

    <div class="card" style="margin-bottom: 24px;">
      <div class="card-title">
        <span>Modified Files & Live Diffs</span>
        <span id="diff-count" style="font-size: 12px; color: var(--text-muted);"></span>
      </div>
      <div class="code-box" id="diff-box">Loading git diffs...</div>
    </div>

    <div class="card">
      <div class="card-title">
        <span>Lifecycle Event Stream</span>
        <span style="font-size: 12px; color: var(--text-muted);" id="events-count"></span>
      </div>
      <div class="timeline" id="events-timeline">
        <div style="color: var(--text-muted); font-size: 13px;">No events recorded yet.</div>
      </div>
    </div>
  </div>

  <script>
    async function refresh() {
      try {
        const res = await fetch('/api/status');
        const data = await res.json();
        
        if (data.checkpoint) {
          document.getElementById('session-objective').innerText = data.checkpoint.objective || 'Autonomous session in progress';
          document.getElementById('session-model').innerText = (data.checkpoint.agent || 'AI Agent') + ' (' + (data.checkpoint.model || 'Auto') + ')';
          document.getElementById('session-id').innerText = data.checkpoint.session_id || 'active';
        }

        const events = data.events || [];
        document.getElementById('events-count').innerText = events.length + ' events';
        if (events.length > 0) {
          const lastEvent = events[events.length - 1];
          if (lastEvent.target) {
            document.getElementById('deploy-target').innerText = lastEvent.target + ' (' + (lastEvent.environment || 'production') + ')';
            document.getElementById('deploy-target-badge').innerText = lastEvent.target;
          }
          if (lastEvent.live_url) {
            document.getElementById('deploy-url').innerHTML = '<a href="' + lastEvent.live_url + '" target="_blank" style="color: var(--primary);">' + lastEvent.live_url + '</a>';
          }
          if (lastEvent.type.includes('success')) {
            document.getElementById('deploy-health').innerHTML = '<span style="color: var(--success); font-weight: 600;">✓ Healthy (HTTP 200)</span>';
          } else if (lastEvent.type.includes('error')) {
            document.getElementById('deploy-health').innerHTML = '<span style="color: var(--danger); font-weight: 600;">✗ Probe Failed: ' + (lastEvent.error_msg || 'Error') + '</span>';
          }

          const timeline = document.getElementById('events-timeline');
          timeline.innerHTML = '';
          events.slice().reverse().forEach(ev => {
            const item = document.createElement('div');
            item.className = 'event-item';
            if (ev.type.includes('error')) item.style.borderLeftColor = 'var(--danger)';
            if (ev.type.includes('success')) item.style.borderLeftColor = 'var(--success)';
            item.innerHTML = '<div class="event-header"><span>' + ev.type + '</span><span style="color: var(--text-muted); font-size: 11px;">' + new Date(ev.timestamp).toLocaleTimeString() + '</span></div><div>' + (ev.details || ev.error_msg || ev.target || '') + '</div>';
            timeline.appendChild(item);
          });
        }

        const diffRes = await fetch('/api/diff');
        const diffText = await diffRes.text();
        document.getElementById('diff-box').innerText = diffText || 'No pending git changes (clean working tree).';
      } catch (err) {
        console.error(err);
      }
    }

    function copyTakeover() {
      navigator.clipboard.writeText('nextdeploy trace takeover -o');
      alert('Copied to clipboard: nextdeploy trace takeover -o');
    }

    refresh();
    setInterval(refresh, 3000);
  </script>
</body>
</html>
`
