//go:build ignore
// +build ignore

package playground


// CaddyManager provides simplified Caddy server configuration management
// Create manager
cm := caddy.New("http://localhost:2019")

// Get current config
config, err := cm.GetConfig(context.Background())
if err != nil {
    panic(err)
}

// Load from file
fileConfig, err := cm.LoadConfig(context.Background(), "/path/to/Caddyfile")
if err != nil {
    panic(err)
}

// Apply new config
err = cm.ApplyConfig(context.Background(), fileConfig)
if err != nil {
    panic(err)
}
