// Public route (listed in nextdeploy.yml protection.public_paths).
//
// Your auth UI/logic goes here — NextDeploy doesn't build your app. The edge
// guard only *verifies* a session cookie on protected paths; issuing it (login,
// password/OAuth, the cookie itself) is yours. See README "Protection" for the
// cookie contract the guard checks.
export default function Login() {
  return (
    <main className="container">
      <div className="status-card" style={{ maxWidth: 440, margin: "60px auto" }}>
        <h1 className="title" style={{ fontSize: "1.8rem", textAlign: "center" }}>Sign In</h1>
        <p style={{ color: "var(--text-secondary)", textAlign: "center", fontSize: "0.95rem" }}>
          This route is public. Guarded routes (e.g. <code>/app</code>) require an authenticated session cookie.
        </p>
        <form style={{ display: "flex", flexDirection: "column", gap: 16, marginTop: 12 }}>
          <div>
            <label className="detail-label" htmlFor="email">Email</label>
            <input
              id="email"
              type="email"
              placeholder="operator@example.com"
              style={{
                width: "100%",
                padding: "10px 14px",
                borderRadius: 8,
                border: "1px solid var(--card-border)",
                background: "var(--bg-secondary)",
                color: "var(--text-primary)",
                marginTop: 6,
              }}
            />
          </div>
          <div>
            <label className="detail-label" htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              placeholder="••••••••"
              style={{
                width: "100%",
                padding: "10px 14px",
                borderRadius: 8,
                border: "1px solid var(--card-border)",
                background: "var(--bg-secondary)",
                color: "var(--text-primary)",
                marginTop: 6,
              }}
            />
          </div>
          <button type="button" className="btn-secondary" style={{ marginTop: 8 }}>
            Continue
          </button>
        </form>
        <div style={{ textAlign: "center", marginTop: 16 }}>
          <a href="/" style={{ color: "var(--accent-primary)", fontSize: "0.9rem", textDecoration: "none" }}>
            ← Back to Home
          </a>
        </div>
      </div>
    </main>
  );
}
