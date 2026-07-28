package prepare

// Config file contents provisioned onto the host. Kept as package constants so
// the Go path and the (legacy) Ansible playbook can be diffed against one
// source of truth instead of drifting apart in two places.

// logrotateCaddy rotates everything Caddy and Coraza append to.
//
// copytruncate because Caddy holds the log file open: a plain rotate renames the
// inode and Caddy keeps writing to the old one until reload. copytruncate copies
// then truncates in place so the running process keeps its handle — the trade is
// a tiny race window (writes between copy and truncate are lost) versus a
// `postrotate systemctl reload caddy` that costs a reload. For access logs the
// race is the cheaper side.
const logrotateCaddy = `/var/log/caddy/*.log {
  daily
  rotate 14
  compress
  delaycompress
  missingok
  notifempty
  copytruncate
  su caddy caddy
}
`

// filterCaddyAuth bans on repeated 401/403 in Caddy's JSON access log.
//
// The lookaheads require both fields on the line without assuming remote_ip
// textually precedes status. JSON guarantees no key order, so the older
// positional regex would have silently stopped matching after a Caddy format
// change — and you'd only find out while under attack.
const filterCaddyAuth = `[Definition]
failregex = ^(?=.*"status":\s*(401|403))(?=.*"remote_ip":\s*"<HOST>").*$
ignoreregex =
`

// filterCaddyWAF bans IPs the Coraza WAF intervened on.
//
// Coraza writes to audit.log, NOT access.log, so before this jail existed the
// WAF caught attacks that fail2ban never saw — two defensive layers that didn't
// talk to each other. Verify the field name against a real line on YOUR
// Coraza/CRS version: trigger a block (curl 'https://host/?x=/etc/passwd'), read
// /var/log/caddy/audit.log, then check with
//
//	fail2ban-regex /var/log/caddy/audit.log /etc/fail2ban/filter.d/caddy-waf.conf
//
// which reports how many lines matched. Several spellings are accepted because
// the serial audit format differs between versions.
const filterCaddyWAF = `[Definition]
failregex = "client_ip":\s*"<HOST>"
            "clientIp":\s*"<HOST>"
            \[client "<HOST>"\]
ignoreregex =
`

const jailCaddy = `[caddy]
enabled  = true
port     = http,https
filter   = caddy-auth
logpath  = /var/log/caddy/access.log
maxretry = 5
bantime  = 3600
`

const jailCaddyWAF = `[caddy-waf]
enabled  = true
port     = http,https
filter   = caddy-waf
logpath  = /var/log/caddy/audit.log
maxretry = 3
bantime  = 7200
`
