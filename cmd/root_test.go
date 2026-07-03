package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/config"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

func TestOJSIngressUpdateSetsRepositoryID(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	input := `services:
  ojs:
    image: libops/ojs:php84
    environment:
      OJS_BASE_URL: "http://localhost"
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	compose, err := corecomponent.LoadComposeFile(path)
	if err != nil {
		t.Fatalf("LoadComposeFile() error = %v", err)
	}
	ctx := &config.Context{
		DockerHostType: config.ContextRemote,
		SSHHostname:    "172.239.194.15",
	}
	update := coretraefik.IngressAppUpdate{
		Domain:  "qa-origin.libops.io",
		BaseURL: "https://qa-origin.libops.io",
		Scheme:  "https",
		HTTPS:   true,
	}
	if err := applyOJSIngressUpdate("ojs")(context.Background(), ctx, compose, update); err != nil {
		t.Fatalf("applyOJSIngressUpdate() error = %v", err)
	}
	if err := compose.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := `OJS_OAI_REPOSITORY_ID: "qa-origin.libops.io"`
	if !strings.Contains(got, want) {
		t.Fatalf("expected compose to contain %q:\n%s", want, got)
	}
}
