export default function Home() {
  return (
    <main style={{ fontFamily: "system-ui", padding: 48 }}>
      <h1>__APP_NAME__</h1>
      <p>Cloudflare fullstack starter — D1/Hyperdrive + R2 + Workers AI, deployed with NextDeploy.</p>
      <ul>
        <li><a href="/login">Login</a> (public)</li>
        <li><a href="/app">Dashboard</a> (guarded by the edge protection layer)</li>
      </ul>
    </main>
  );
}
