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

// mainCaddyfile is the seed for /etc/caddy/Caddyfile.
//
// The daemon's CaddyManager.EnsureMainCaddyfile converges on exactly this shape
// at deploy time, but Caddy has to START before the first deploy — and it won't
// start against the distro's default welcome-page Caddyfile once a nextdeploy.d
// fragment lands, nor against no file at all. Seeding it here means the service
// is up and valid from the end of prepare.
//
// `order coraza_waf first` is what makes every generated fragment parseable;
// without the Coraza module present, Caddy rejects this file outright, which is
// why the module step runs before the service is started.
const mainCaddyfile = `{
	order coraza_waf first
}

import /etc/caddy/nextdeploy.d/*.caddy
`

// caddyUnit is written ONLY when the package manager didn't supply one (the
// static-binary fallback path on hosts where neither the Cloudsmith repo nor
// COPR is reachable). The apt and COPR packages ship their own unit and we
// leave it alone.
const caddyUnit = `[Unit]
Description=Caddy web server
Documentation=https://caddyserver.com/docs/
After=network.target network-online.target
Requires=network-online.target

[Service]
User=root
ExecStart=/usr/local/bin/caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
TimeoutStopSec=5s
Restart=on-failure
RestartSec=5s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`

// daemonUnit runs nextdeployd itself.
//
// RuntimeDirectory/LogsDirectory are the load-bearing lines: systemd creates
// /run/nextdeployd (which holds the control socket) and /var/log/nextdeployd
// (audit log) with group `nextdeploy` before ExecStart, and cleans the runtime
// dir up on stop. Without them the daemon starts, fails to bind its socket in a
// directory that doesn't exist, and every CLI command reports "is nextdeployd
// running?" against a process that is, in fact, running.
//
// After/Wants caddy: the daemon reloads Caddy on every activation, so ordering
// it after the web server avoids a first-deploy race on a freshly booted host.
const daemonUnit = `[Unit]
Description=NextDeploy Daemon
Documentation=https://nextdeploy.org/docs
After=network.target caddy.service
Wants=caddy.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/nextdeployd --foreground=true
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nextdeployd

RuntimeDirectory=nextdeployd
RuntimeDirectoryMode=0755
RuntimeDirectoryGroup=nextdeploy
LogsDirectory=nextdeployd
LogsDirectoryMode=0755
LogsDirectoryGroup=nextdeploy

# The daemon writes to /etc/caddy, /etc/systemd/system, /opt/nextdeploy,
# /run/nextdeployd and /var/log/nextdeployd, and chowns release trees to the
# service user — so the bounding set stays broad on purpose.
NoNewPrivileges=yes
CapabilityBoundingSet=CAP_DAC_OVERRIDE CAP_NET_BIND_SERVICE CAP_CHOWN CAP_FOWNER

[Install]
WantedBy=multi-user.target
`
