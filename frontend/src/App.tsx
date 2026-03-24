function App() {
  return (
    <div className="dark min-h-screen bg-background p-8">
      <div className="container">
        <div className="page-header">
          <h1 className="title">Metio</h1>
          <p className="subtitle">Minecraft Server Controller</p>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="card-title">
              Server Status
              <span className="badge badge-online">Online</span>
            </h2>
          </div>
          <div className="card-content">
            <div className="stats-grid">
              <div className="stat">
                <span className="stat-label">Status</span>
                <span className="stat-value">Running</span>
              </div>
              <div className="stat">
                <span className="stat-label">Players</span>
                <span className="stat-value">3/20</span>
              </div>
              <div className="stat">
                <span className="stat-label">Uptime</span>
                <span className="stat-value">2h 34m</span>
              </div>
              <div className="stat">
                <span className="stat-label">Memory</span>
                <span className="stat-value">2.4 GB</span>
              </div>
            </div>

            <div className="separator" />

            <div className="controls">
              <button className="btn btn-red">Stop Server</button>
              <button className="btn btn-outline">Copy IP</button>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="card-title">Badge Examples</h2>
          </div>
          <div className="card-content">
            <div className="flex gap-2">
              <span className="badge badge-online">Online</span>
              <span className="badge badge-offline">Offline</span>
              <span className="badge badge-transitioning">Starting...</span>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="card-title">Button Examples</h2>
          </div>
          <div className="card-content">
            <div className="controls">
              <button className="btn btn-green">Start</button>
              <button className="btn btn-red">Stop</button>
              <button className="btn btn-outline">Settings</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default App
