package codexreport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanConfigRedactsSensitiveValues(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config.toml")
	content := []byte(`
model = "gpt-5.4"
sandbox_mode = "workspace-write"
experimental_bearer_token = "secret-token"

[model_providers.custom]
base_url = "https://example.com"
env_key = "OPENAI_API_KEY"

[mcp_servers.github]
command = "github-mcp"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	got := scanConfigFile(path, home)
	if !got.Exists {
		t.Fatal("expected config to exist")
	}
	if got.Keys["model"] != "gpt-5.4" {
		t.Fatalf("model=%q", got.Keys["model"])
	}
	if got.Keys["mcp_server:github"] != "configured" {
		t.Fatalf("missing MCP summary: %#v", got.Keys)
	}
	for _, value := range got.Keys {
		if value == "secret-token" || value == "OPENAI_API_KEY" {
			t.Fatalf("sensitive value leaked: %#v", got.Keys)
		}
	}
	if len(got.Secrets) == 0 {
		t.Fatal("expected sensitive keys to be listed")
	}
}
