import os
import subprocess

PUBLIC_DIR = os.path.abspath("frontend/public")
APP_DIR = os.path.abspath("frontend/app")
os.makedirs(PUBLIC_DIR, exist_ok=True)
os.makedirs(APP_DIR, exist_ok=True)

FONT_PATH = "/System/Library/Fonts/Supplemental/Arial.ttf"

# 1. logo-icon.svg (512x512) - High-precision vector icon
ICON_SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0b1329" />
      <stop offset="50%" stop-color="#050a17" />
      <stop offset="100%" stop-color="#020617" />
    </linearGradient>
    <linearGradient id="border" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#34d399" stop-opacity="0.7" />
      <stop offset="45%" stop-color="#06b6d4" stop-opacity="0.4" />
      <stop offset="100%" stop-color="#1e293b" stop-opacity="0.8" />
    </linearGradient>
    <linearGradient id="frame" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#059669" />
    </linearGradient>
    <linearGradient id="bolt" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#a7f3d0" />
      <stop offset="25%" stop-color="#34d399" />
      <stop offset="70%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
    <linearGradient id="innerBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0f172a" stop-opacity="0.95" />
      <stop offset="100%" stop-color="#020617" stop-opacity="0.98" />
    </linearGradient>
    <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="10" result="blur" />
      <feComposite in="SourceGraphic" in2="blur" operator="over" />
    </filter>
  </defs>

  <!-- Background squircle -->
  <rect x="16" y="16" width="480" height="480" rx="112" fill="url(#bg)" stroke="url(#border)" stroke-width="6" />

  <!-- Server container chassis -->
  <rect x="96" y="96" width="320" height="320" rx="56" fill="url(#innerBg)" stroke="url(#frame)" stroke-width="16" />

  <!-- Status indicators -->
  <circle cx="148" cy="148" r="10" fill="#34d399" />
  <circle cx="180" cy="148" r="10" fill="#38bdf8" />
  <rect x="300" y="143" width="64" height="10" rx="5" fill="#334155" />

  <!-- Lightning Deploy Bolt with high-energy glow -->
  <path d="M275 185 L215 270 L250 270 L220 360 L305 250 L262 250 Z" fill="url(#bolt)" filter="url(#glow)" />
</svg>"""

# 2. icon.svg (Modern tab vector icon with adaptive dark/light mode)
TAB_ICON_SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <defs>
    <linearGradient id="tabBolt" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#6ee7b7" />
      <stop offset="50%" stop-color="#34d399" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
    <linearGradient id="tabChassis" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#10b981" />
      <stop offset="0%" stop-color="#059669" />
    </linearGradient>
  </defs>
  <rect x="4" y="4" width="120" height="120" rx="28" fill="#020617" stroke="#1e293b" stroke-width="2"/>
  <rect x="24" y="24" width="80" height="80" rx="16" fill="#0f172a" stroke="url(#tabChassis)" stroke-width="6"/>
  <circle cx="38" cy="38" r="3.5" fill="#34d399"/>
  <circle cx="48" cy="38" r="3.5" fill="#38bdf8"/>
  <path d="M68 46 L53 68 L62 68 L54 91 L76 63 L65 63 Z" fill="url(#tabBolt)"/>
</svg>"""

# 3. logo.svg (Full horizontal brand logo 800x200)
LOGO_SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 200" width="800" height="200">
  <defs>
    <linearGradient id="logoBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0b1329" />
      <stop offset="100%" stop-color="#020617" />
    </linearGradient>
    <linearGradient id="logoBorder" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#34d399" stop-opacity="0.4" />
      <stop offset="100%" stop-color="#1e293b" stop-opacity="0.6" />
    </linearGradient>
    <linearGradient id="logoChassis" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#059669" />
    </linearGradient>
    <linearGradient id="logoBolt" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#a7f3d0" />
      <stop offset="30%" stop-color="#34d399" />
      <stop offset="80%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
    <linearGradient id="deployGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#34d399" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
    <filter id="logoGlow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="4" result="blur" />
      <feComposite in="SourceGraphic" in2="blur" operator="over" />
    </filter>
  </defs>

  <!-- Background banner -->
  <rect width="800" height="200" rx="28" fill="url(#logoBg)" stroke="url(#logoBorder)" stroke-width="2" />

  <!-- Logo Icon Mark (left) -->
  <g transform="translate(48, 36)">
    <!-- Squircle badge -->
    <rect x="0" y="0" width="128" height="128" rx="28" fill="#090e1c" stroke="#1e293b" stroke-width="2" />
    <!-- Server chassis -->
    <rect x="20" y="20" width="88" height="88" rx="18" fill="#0f172a" stroke="url(#logoChassis)" stroke-width="5" />
    <!-- Status LEDs -->
    <circle cx="35" cy="35" r="3.5" fill="#34d399" />
    <circle cx="46" cy="35" r="3.5" fill="#38bdf8" />
    <rect x="76" y="33" width="18" height="4" rx="2" fill="#334155" />
    <!-- Bolt -->
    <path d="M68 44 L52 68 L61 68 L53 92 L76 63 L65 63 Z" fill="url(#logoBolt)" filter="url(#logoGlow)" />
  </g>

  <!-- Typography -->
  <g transform="translate(208, 108)">
    <text font-family="Arial, Helvetica, sans-serif" font-weight="800" font-size="64" letter-spacing="-1.5">
      <tspan fill="#f8fafc">Next</tspan><tspan fill="url(#deployGrad)">Deploy</tspan>
    </text>
  </g>

  <!-- Subtitle Tag -->
  <g transform="translate(212, 148)">
    <text font-family="Arial, Helvetica, sans-serif" font-weight="600" font-size="16" fill="#94a3b8" letter-spacing="4">
      SELF-HOSTED NEXT.JS ENGINE
    </text>
  </g>

  <!-- Edge Pill Badge (right) -->
  <g transform="translate(620, 48)">
    <rect width="132" height="30" rx="15" fill="#10b981" fill-opacity="0.12" stroke="#10b981" stroke-opacity="0.3" stroke-width="1.5" />
    <circle cx="16" cy="15" r="4" fill="#34d399" />
    <text x="28" y="19.5" font-family="Arial, Helvetica, sans-serif" font-size="11" font-weight="700" fill="#34d399" letter-spacing="0.5">
      v0.20 READY
    </text>
  </g>
</svg>"""

# 4. OpenGraph Social Card (1200x630)
OG_SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" width="1200" height="630">
  <defs>
    <radialGradient id="ogGlow1" cx="20%" cy="20%" r="60%">
      <stop offset="0%" stop-color="#10b981" stop-opacity="0.25" />
      <stop offset="100%" stop-color="#020617" stop-opacity="0" />
    </radialGradient>
    <radialGradient id="ogGlow2" cx="80%" cy="80%" r="50%">
      <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.2" />
      <stop offset="100%" stop-color="#020617" stop-opacity="0" />
    </radialGradient>
    <linearGradient id="ogCard" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0b1329" stop-opacity="0.9" />
      <stop offset="100%" stop-color="#020617" stop-opacity="0.95" />
    </linearGradient>
    <linearGradient id="textGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#34d399" />
      <stop offset="100%" stop-color="#38bdf8" />
    </linearGradient>
    <linearGradient id="ogBolt" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#a7f3d0" />
      <stop offset="30%" stop-color="#34d399" />
      <stop offset="80%" stop-color="#10b981" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
  </defs>

  <!-- Deep base background -->
  <rect width="1200" height="630" fill="#020617" />
  <rect width="1200" height="630" fill="url(#ogGlow1)" />
  <rect width="1200" height="630" fill="url(#ogGlow2)" />

  <!-- Subtle grid pattern -->
  <g stroke="#1e293b" stroke-width="1" stroke-opacity="0.35">
    <line x1="0" y1="100" x2="1200" y2="100" />
    <line x1="0" y1="200" x2="1200" y2="200" />
    <line x1="0" y1="300" x2="1200" y2="300" />
    <line x1="0" y1="400" x2="1200" y2="400" />
    <line x1="0" y1="500" x2="1200" y2="500" />
    <line x1="200" y1="0" x2="200" y2="630" />
    <line x1="400" y1="0" x2="400" y2="630" />
    <line x1="600" y1="0" x2="600" y2="630" />
    <line x1="800" y1="0" x2="800" y2="630" />
    <line x1="1000" y1="0" x2="1000" y2="630" />
  </g>

  <!-- Outer frame border -->
  <rect x="24" y="24" width="1152" height="582" rx="24" fill="none" stroke="#1e293b" stroke-width="2" />

  <!-- Logo Mark + Header -->
  <g transform="translate(80, 80)">
    <!-- Squircle badge -->
    <rect x="0" y="0" width="88" height="88" rx="22" fill="#0b1329" stroke="#34d399" stroke-width="3" stroke-opacity="0.8" />
    <!-- Server chassis -->
    <rect x="14" y="14" width="60" height="60" rx="12" fill="#0f172a" stroke="#10b981" stroke-width="4" />
    <!-- Status LEDs -->
    <circle cx="25" cy="25" r="3" fill="#34d399" />
    <circle cx="34" cy="25" r="3" fill="#38bdf8" />
    <!-- Bolt -->
    <path d="M47 31 L36 47 L43 47 L37 64 L53 43 L45 43 Z" fill="url(#ogBolt)" />

    <!-- Wordmark -->
    <text x="110" y="58" font-family="Arial, Helvetica, sans-serif" font-weight="800" font-size="48" letter-spacing="-1">
      <tspan fill="#f8fafc">Next</tspan><tspan fill="url(#textGrad)">Deploy</tspan>
    </text>
    <text x="112" y="80" font-family="Arial, Helvetica, sans-serif" font-size="13" font-weight="600" fill="#94a3b8" letter-spacing="3">
      OPEN-SOURCE INFRASTRUCTURE ENGINE
    </text>
  </g>

  <!-- Big Hero Title -->
  <g transform="translate(80, 240)">
    <text font-family="Arial, Helvetica, sans-serif" font-weight="800" font-size="52" fill="#ffffff" letter-spacing="-1.5">
      Deploy Next.js to Your Own VPS,
    </text>
    <text y="70" font-family="Arial, Helvetica, sans-serif" font-weight="800" font-size="52" fill="url(#textGrad)" letter-spacing="-1.5">
      Cloudflare Workers &amp; AWS Lambda
    </text>
  </g>

  <!-- Sub-headline -->
  <g transform="translate(80, 390)">
    <text font-family="Arial, Helvetica, sans-serif" font-weight="400" font-size="23" fill="#94a3b8">
      Zero lock-in · Atomic zero-downtime port-swaps · Automatic Let's Encrypt SSL · WAF &amp; Agent Trace
    </text>
  </g>

  <!-- Badges footer -->
  <g transform="translate(80, 480)">
    <!-- Terminal install pill -->
    <rect width="520" height="56" rx="14" fill="#0b1329" stroke="#334155" stroke-width="1.5" />
    <text x="24" y="35" font-family="Arial, Helvetica, sans-serif" font-size="18" fill="#34d399" font-weight="600">$</text>
    <text x="46" y="35" font-family="Arial, Helvetica, sans-serif" font-size="17" fill="#e2e8f0">curl -sSf https://nextdeploy.org/install.sh | sh</text>

    <!-- Platform pills -->
    <g transform="translate(560, 0)">
      <rect x="0" y="0" width="130" height="56" rx="14" fill="#10b981" fill-opacity="0.1" stroke="#10b981" stroke-opacity="0.3" stroke-width="1.5" />
      <text x="65" y="34" font-family="Arial, Helvetica, sans-serif" font-size="16" font-weight="700" fill="#34d399" text-anchor="middle">Linux VPS</text>

      <rect x="145" y="0" width="145" height="56" rx="14" fill="#06b6d4" fill-opacity="0.1" stroke="#06b6d4" stroke-opacity="0.3" stroke-width="1.5" />
      <text x="217" y="34" font-family="Arial, Helvetica, sans-serif" font-size="16" font-weight="700" fill="#38bdf8" text-anchor="middle">Cloudflare</text>

      <rect x="305" y="0" width="155" height="56" rx="14" fill="#f59e0b" fill-opacity="0.1" stroke="#f59e0b" stroke-opacity="0.3" stroke-width="1.5" />
      <text x="382" y="34" font-family="Arial, Helvetica, sans-serif" font-size="16" font-weight="700" fill="#fbbf24" text-anchor="middle">AWS Lambda</text>
    </g>
  </g>
</svg>"""

# 5. Windows Wide Tile (mstile-310x150.png @ 620x300)
WIDE_TILE_SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 620 300" width="620" height="300">
  <defs>
    <linearGradient id="wtBg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0b1329" />
      <stop offset="100%" stop-color="#020617" />
    </linearGradient>
    <linearGradient id="wtBolt" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#a7f3d0" />
      <stop offset="40%" stop-color="#34d399" />
      <stop offset="100%" stop-color="#06b6d4" />
    </linearGradient>
  </defs>
  <rect width="620" height="300" fill="url(#wtBg)" />
  <g transform="translate(60, 50)">
    <rect x="0" y="0" width="200" height="200" rx="44" fill="#0f172a" stroke="#10b981" stroke-width="8" />
    <circle cx="36" cy="36" r="6" fill="#34d399" />
    <circle cx="56" cy="36" r="6" fill="#38bdf8" />
    <path d="M108 55 L74 110 L94 110 L80 155 L124 98 L104 98 Z" fill="url(#wtBolt)" />
  </g>
  <text x="290" y="145" font-family="Arial, Helvetica, sans-serif" font-weight="800" font-size="52" fill="#ffffff">
    Next<tspan fill="#34d399">Deploy</tspan>
  </text>
  <text x="292" y="195" font-family="Arial, Helvetica, sans-serif" font-size="16" fill="#94a3b8" letter-spacing="3">
    SELF-HOSTED NEXT.JS
  </text>
</svg>"""

# Write all vector files
with open(os.path.join(PUBLIC_DIR, "logo-icon.svg"), "w") as f:
    f.write(ICON_SVG)

with open(os.path.join(PUBLIC_DIR, "icon.svg"), "w") as f:
    f.write(TAB_ICON_SVG)

with open(os.path.join(PUBLIC_DIR, "logo.svg"), "w") as f:
    f.write(LOGO_SVG)

with open("/tmp/nd-icon.svg", "w") as f:
    f.write(ICON_SVG)

with open("/tmp/nd-logo.svg", "w") as f:
    f.write(LOGO_SVG)

with open("/tmp/nd-og.svg", "w") as f:
    f.write(OG_SVG)

with open("/tmp/nd-wide.svg", "w") as f:
    f.write(WIDE_TILE_SVG)

print("Vector SVGs written successfully.")

def run(cmd):
    print("Running:", cmd)
    subprocess.check_call(cmd, shell=True)

# 1. Favicons
run(f"magick -background none /tmp/nd-icon.svg -resize 16x16 {PUBLIC_DIR}/favicon-16x16.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 32x32 {PUBLIC_DIR}/favicon-32x32.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 48x48 {PUBLIC_DIR}/favicon-48x48.png")

# Multi-layer favicon.ico containing 16x16, 32x32, 48x48 (in app/ only to avoid public/ conflict)
run(f"magick {PUBLIC_DIR}/favicon-16x16.png {PUBLIC_DIR}/favicon-32x32.png {PUBLIC_DIR}/favicon-48x48.png {APP_DIR}/favicon.ico")

# 2. Apple Touch Icons
run(f"magick -background none /tmp/nd-icon.svg -resize 180x180 {PUBLIC_DIR}/apple-touch-icon.png")
run(f"cp {PUBLIC_DIR}/apple-touch-icon.png {PUBLIC_DIR}/apple-touch-icon-precomposed.png")
run(f"cp {PUBLIC_DIR}/apple-touch-icon.png {APP_DIR}/apple-icon.png")

# 3. Android / PWA Icons
run(f"magick -background none /tmp/nd-icon.svg -resize 192x192 {PUBLIC_DIR}/android-chrome-192x192.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 512x512 {PUBLIC_DIR}/android-chrome-512x512.png")
run(f"cp {PUBLIC_DIR}/android-chrome-192x192.png {PUBLIC_DIR}/icon-192.png")
run(f"cp {PUBLIC_DIR}/android-chrome-512x512.png {PUBLIC_DIR}/icon-512.png")
run(f"cp {PUBLIC_DIR}/android-chrome-512x512.png {APP_DIR}/icon.png")

# Maskable icon with 10% safe area padding
run(f"magick {PUBLIC_DIR}/android-chrome-512x512.png -gravity center -background '#020617' -extent 570x570 -resize 512x512 {PUBLIC_DIR}/maskable-icon-512x512.png")

# 4. Windows MSApplication Tiles
run(f"magick -background none /tmp/nd-icon.svg -resize 70x70 {PUBLIC_DIR}/mstile-70x70.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 144x144 {PUBLIC_DIR}/mstile-144x144.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 150x150 {PUBLIC_DIR}/mstile-150x150.png")
run(f"magick -font {FONT_PATH} -background none /tmp/nd-wide.svg -resize 310x150 {PUBLIC_DIR}/mstile-310x150.png")
run(f"magick -background none /tmp/nd-icon.svg -resize 310x310 {PUBLIC_DIR}/mstile-310x310.png")

# 5. OpenGraph & Twitter Cards
run(f"magick -font {FONT_PATH} /tmp/nd-og.svg -resize 1200x630 -strip -depth 8 {PUBLIC_DIR}/og.png")
run(f"cp {PUBLIC_DIR}/og.png {APP_DIR}/opengraph-image.png")
run(f"cp {PUBLIC_DIR}/og.png {APP_DIR}/twitter-image.png")

# 6. High-Res Logo PNG
run(f"magick -font {FONT_PATH} -background none /tmp/nd-logo.svg -resize 800x200 {PUBLIC_DIR}/logo.png")

# 7. browserconfig.xml
BROWSER_CONFIG = """<?xml version="1.0" encoding="utf-8"?>
<browserconfig>
  <msapplication>
    <tile>
      <square70x70logo src="/mstile-70x70.png"/>
      <square150x150logo src="/mstile-150x150.png"/>
      <wide310x150logo src="/mstile-310x150.png"/>
      <square310x310logo src="/mstile-310x310.png"/>
      <TileColor>#020617</TileColor>
    </tile>
  </msapplication>
</browserconfig>
"""
with open(os.path.join(PUBLIC_DIR, "browserconfig.xml"), "w") as f:
    f.write(BROWSER_CONFIG.strip() + "\\n")

# 8. site.webmanifest
WEB_MANIFEST = """{
  "name": "NextDeploy - Self-Hosted Next.js Deployment Platform",
  "short_name": "NextDeploy",
  "description": "Deploy Next.js to your own Linux VPS, Cloudflare Workers, or AWS Lambda with zero lock-in.",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#020617",
  "theme_color": "#10b981",
  "icons": [
    {
      "src": "/favicon-16x16.png",
      "sizes": "16x16",
      "type": "image/png"
    },
    {
      "src": "/favicon-32x32.png",
      "sizes": "32x32",
      "type": "image/png"
    },
    {
      "src": "/android-chrome-192x192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/android-chrome-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/maskable-icon-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "maskable"
    }
  ]
}
"""
with open(os.path.join(PUBLIC_DIR, "site.webmanifest"), "w") as f:
    f.write(WEB_MANIFEST.strip() + "\\n")

print("All branding, tile, favicon, and OG assets generated successfully!")
