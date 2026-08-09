package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/plugin"
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

func TestCreateDefinitionLifecycleContract(t *testing.T) {
	t.Parallel()
	spec := createDefinition()
	if spec.DockerComposeBranch != "v1.2.1" {
		t.Fatalf("OJS template revision = %q, want immutable v1.2.1", spec.DockerComposeBranch)
	}
	if len(spec.Images) != 1 || spec.Images[0].Image != "libops/ojs:3.5.0-5-php84" || spec.Images[0].BuildPolicy != plugin.BuildPolicyAlways {
		t.Fatalf("unexpected OJS image contract: %+v", spec.Images)
	}
	if len(spec.DockerComposeUp) != 1 || !strings.Contains(spec.DockerComposeUp[0], "--wait --wait-timeout 600") {
		t.Fatalf("create must wait for service health before reporting ready: %+v", spec.DockerComposeUp)
	}
	for _, volume := range spec.InitVolumes {
		if volume.Name == "ojs-cache" {
			t.Fatalf("disposable OJS cache must not be lifecycle state: %+v", spec.InitVolumes)
		}
	}
	rollout := strings.Join(spec.DockerComposeRollout, "\n")
	if !strings.Contains(rollout, "php tools/upgrade.php upgrade") || strings.Contains(rollout, "|| true") || strings.Contains(rollout, "skipped or failed") {
		t.Fatalf("OJS schema migration must run and fail hard:\n%s", rollout)
	}
	assertMigrationBeforeWait(t, spec.DockerComposeRollout, "php tools/upgrade.php upgrade")

	sdk := plugin.NewSDK(plugin.Metadata{Name: "ojs"})
	RegisterCommands(sdk)
	for _, definition := range sdk.LocalComponentDefinitions() {
		if definition.Name == "dev-mode" {
			t.Fatal("dev-mode must not mask bundled OJS plugin directories")
		}
	}
}

func assertMigrationBeforeWait(t *testing.T, commands []string, migration string) {
	t.Helper()
	for index, command := range commands {
		if !strings.Contains(command, migration) {
			continue
		}
		if strings.Contains(command, "||") {
			t.Fatalf("migration must fail hard: %+v", commands)
		}
		if index < 2 || commands[index-1] != "docker compose exec -T ojs /usr/local/bin/sitectl-ojs-rollout-readiness" {
			t.Fatalf("service must complete bounded setup readiness before migration: %+v", commands)
		}
		initialStart := commands[index-2]
		wantInitialStart := "docker compose up --remove-orphans --pull missing --quiet-pull -d ojs"
		if initialStart != wantInitialStart ||
			!strings.HasSuffix(initialStart, " -d ojs") ||
			strings.Contains(initialStart, "--wait") {
			t.Fatalf("initial rollout start must target only OJS without waiting: %q", initialStart)
		}
		if index+1 >= len(commands) {
			t.Fatalf("bounded final health wait must follow migration: %+v", commands)
		}
		finalStart := commands[index+1]
		wantFinalStart := "docker compose up --remove-orphans --wait --wait-timeout 600 --pull missing --quiet-pull -d"
		if finalStart != wantFinalStart ||
			!strings.Contains(finalStart, "--wait --wait-timeout 600") ||
			!strings.HasSuffix(finalStart, " -d") ||
			strings.Contains(finalStart, "||") {
			t.Fatalf("final rollout start must wait for the full stack and fail hard: %q", finalStart)
		}
		return
	}
	t.Fatalf("migration %q not found: %+v", migration, commands)
}
