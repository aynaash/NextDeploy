"use client";

import { useEffect, useState } from "react";

export default function Home() {
  const [health, setHealth] = useState<{ status: string; uptime: number; platform: string } | null>(null);
  const [latency, setLatency] = useState<number | null>(null);
  const [testing, setTesting] = useState(false);

  const checkHealth = async () => {
    setTesting(true);
    const start = performance.now();
    try {
      const res = await fetch("/api/health");
      const data = await res.json();
      setHealth(data);
      setLatency(Math.round(performance.now() - start));
    } catch {
      setHealth({ status: "error", uptime: 0, platform: "unknown" });
    } finally {
      setTesting(false);
    }
  };

  useEffect(() => {
    checkHealth();
  }, []);

  return (
    <main className="container">
      <header className="header">
        <div className="logo-badge">
          <span className="logo-icon">▲</span>
          <span className="logo-text">NextDeploy</span>
        </div>
        <div className="status-pill">
          <span className="status-dot"></span>
          <span>{health ? health.status.toUpperCase() : "OPERATIONAL"}</span>
        </div>
      </header>

      <section className="hero">
        <div className="badge">Next.js 15 + React 19 Fullstack Starter</div>
        <h1 className="title">__APP_NAME__</h1>
        <p className="subtitle">
          Production-ready fullstack application configured for{" "}
          <strong>__PLATFORM_NAME__</strong>. Optimized for instant rollbacks,
          zero-downtime shipping, and edge performance.
        </p>
      </section>

      <section className="card-grid">
        <div className="card">
          <div className="card-icon">⚡</div>
          <h3>Instant Deployments</h3>
          <p>Ship atomic, zero-downtime updates in one command with <code>nextdeploy ship</code>.</p>
        </div>

        <div className="card">
          <div className="card-icon">🛡️</div>
          <h3>Edge Protection</h3>
          <p>Built-in security, secrets encryption with AES-256-GCM, and rate limiting.</p>
        </div>

        <div className="card">
          <div className="card-icon">🔍</div>
          <h3>Health & Telemetry</h3>
          <p>Live healthcheck endpoints, runtime observability, and automated rollbacks.</p>
        </div>
      </section>

      <section className="status-section">
        <div className="status-card">
          <div className="status-header">
            <h3>Live Healthcheck (/api/health)</h3>
            <button className="btn-secondary" onClick={checkHealth} disabled={testing}>
              {testing ? "Testing..." : "Ping API"}
            </button>
          </div>
          <div className="status-details">
            <div className="detail-item">
              <span className="detail-label">Status:</span>
              <span className="detail-value highlight">{health?.status || "loading..."}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Platform:</span>
              <span className="detail-value">{health?.platform || "__PLATFORM_NAME__"}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Latency:</span>
              <span className="detail-value">{latency !== null ? `${latency}ms` : "--"}</span>
            </div>
          </div>
        </div>
      </section>

      <section className="quickstart">
        <h2>Quick Start</h2>
        <div className="terminal">
          <pre>
            <code>
{`# 1. Start local development
npm run dev

# 2. Preview deployment plan
nextdeploy plan

# 3. Ship to __PLATFORM_NAME__
nextdeploy ship`}
            </code>
          </pre>
        </div>
      </section>

      <footer className="footer">
        <p>Built with NextDeploy by <a href="https://github.com/aynaash" target="_blank" rel="noreferrer">aynaash (Hersi)</a>.</p>
      </footer>
    </main>
  );
}
