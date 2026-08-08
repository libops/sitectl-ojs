package cmd

import (
	"context"

	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/plugin"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

const (
	createRepo   = "https://github.com/libops/ojs"
	createBranch = "v1.0.0"
	pluginName   = "ojs"
	defaultPath  = "./ojs"
)

func createDefinition() plugin.CreateSpec {
	return plugin.CreateSpec{
		Name:                "default",
		Description:         "Create an Open Journal Systems stack",
		Default:             true,
		MinCPUCores:         2,
		MinMemory:           "4 GiB",
		MinDiskSpace:        "20 GiB",
		DockerComposeRepo:   createRepo,
		DockerComposeBranch: createBranch,
		DockerComposeBuild: []string{
			"docker compose pull --ignore-buildable",
			"docker compose build",
		},
		Images: []plugin.ComposeImageSpec{
			{Service: "ojs", Image: "libops/ojs:3.5.0-5-php84", BuildPolicy: plugin.BuildPolicyAlways},
		},
		DockerComposeInit: []string{
			"mkdir -p ./secrets",
			"docker compose run --rm init",
		},
		InitArtifacts: []plugin.InitArtifact{
			{Path: "secrets/DB_ROOT_PASSWORD"},
			{Path: "secrets/OJS_DB_PASSWORD"},
			{Path: "secrets/OJS_API_KEY_SECRET"},
			{Path: "secrets/OJS_SALT"},
			{Path: "secrets/OJS_ADMIN_PASSWORD"},
			{Path: "secrets/OJS_SECRET_KEY"},
		},
		InitVolumes: []plugin.InitVolume{
			{Name: "mariadb-data"},
			{Name: "ojs-files"},
			{Name: "ojs-public"},
		},
		DockerComposeUp: []string{
			"docker compose up --remove-orphans --wait --wait-timeout 600 -d",
		},
		DockerComposeDown: []string{"docker compose down"},
		DockerComposeRollout: []string{
			"docker compose pull --ignore-buildable --quiet || docker compose pull --ignore-buildable",
			"docker compose build --pull",
			"mkdir -p ./secrets",
			"docker compose run --rm init",
			"docker compose up --remove-orphans --pull missing --quiet-pull -d ojs",
			"docker compose exec -T ojs sh -c 'attempt=0; until test -f /installed; do attempt=$((attempt + 1)); if [ \"$attempt\" -ge 150 ]; then echo \"OJS did not become ready for database migration within 5 minutes\" >&2; exit 1; fi; sleep 2; done'",
			"docker compose exec -T ojs php tools/upgrade.php upgrade",
			"docker compose up --remove-orphans --wait --wait-timeout 600 --pull missing --quiet-pull -d",
		},
	}
}

// RegisterCommands registers OJS commands with the plugin SDK.
func RegisterCommands(s *plugin.SDK) {
	s.SetComposeProjectDiscovery(plugin.ComposeProjectDiscovery{
		RequiredServices: []string{"ojs"},
		Reason:           "ojs service",
	})
	s.RegisterComposeTemplateCreateRunner(createDefinition(), plugin.ComposeTemplateCreateOptions{
		DefaultPath:   defaultPath,
		DefaultPlugin: pluginName,
		ReadyMessage:  "OJS is ready for use through sitectl.",
	})
	registerApplicationComponents(s, "OJS", "ojs")
	s.RegisterHealthcheckRunner(ojsHealthcheckRunner)
	s.RegisterVerifyRunner(&ojsVerifyRunner{sdk: s})
	s.RegisterIngressRouteProvider(plugin.StandardComposeWebIngressRoutesWithOptions(plugin.StandardComposeWebIngressOptions{
		AppService: "ojs",
		Router:     "ojs-web",
	}))
	registerOJSCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	ingress, err := coretraefik.Ingress(coretraefik.IngressOptions{
		AppService:      appService,
		HTTPEntrypoint:  "web",
		HTTPSEntrypoint: "websecure",
		AppEnvDeletes:   []string{"OJS_ALLOWED_HOSTS", "OJS_BASE_URL", "OJS_ENABLE_HTTPS"},
		AppUpdate:       applyOJSIngressUpdate(appService),
	})
	if err != nil {
		panic(err)
	}
	s.RegisterServiceComponents(plugin.ServiceComponentRegistryOptions{
		DisplayName: displayName,
		Components:  []corecomponent.ComposeServiceComponent{ingress},
	})
}

func applyOJSIngressUpdate(appService string) coretraefik.IngressAppUpdateFunc {
	return func(_ context.Context, _ *config.Context, compose *corecomponent.ComposeFile, update coretraefik.IngressAppUpdate) error {
		return compose.SetServiceEnv(appService, "OJS_OAI_REPOSITORY_ID", update.Domain)
	}
}
