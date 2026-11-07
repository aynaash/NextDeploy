// Protected route: nextdeploy.yml sets protection.protected_paths: ["/app/*"],
// so NextDeploy's edge guard runs before this renders and 302s unauthenticated
// requests to /login.
export default function Dashboard() {
  return (
    <main className="container">
      <header className="header">
        <div className="logo-badge">
          <span className="logo-icon">▲</span>
          <span className="logo-text">__APP_NAME__ Dashboard</span>
        </div>
        <div className="status-pill">
          <span className="status-dot"></span>
          <span>GUARDED BY EDGE</span>
        </div>
      </header>

      <div className="card" style={{ marginTop: 24 }}>
        <h3>Protected Route Verified</h3>
        <p style={{ marginTop: 8 }}>
          You have successfully reached a route protected by NextDeploy's Edge Protection layer.
          Requests without a valid signed session cookie are blocked at the Cloudflare edge before
          hitting your compute origin.
        </p>
        <div style={{ marginTop: 20 }}>
          <a href="/" className="btn-secondary" style={{ textDecoration: "none", display: "inline-block" }}>
            ← Back to Home
          </a>
        </div>
      </div>
    </main>
  );
}
