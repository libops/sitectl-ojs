package cmd

import (
	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/plugin"
	coredevmode "github.com/libops/sitectl/pkg/services/devmode"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

const (
	createRepo   = "https://github.com/libops/ojs"
	createBranch = "main"
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
			"docker compose build --pull",
		},
		Images: []plugin.ComposeImageSpec{
			{Service: "ojs", Image: "libops/ojs:nginx-1.30.3-php84", BuildPolicy: plugin.BuildPolicyIfNotPresent},
		},
		DockerComposeInit: []string{
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
		DockerComposeUp: []string{
			"docker compose up --remove-orphans -d",
		},
		DockerComposeDown:    []string{"docker compose down"},
		DockerComposeRollout: []string{"./scripts/rollout.sh"},
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
	s.RegisterHealthcheckRunner(ojsHealthcheckRunner{})
	registerOJSCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	reverseProxy, err := coretraefik.ReverseProxy(coretraefik.ReverseProxyOptions{AppService: appService})
	if err != nil {
		panic(err)
	}
	uploadLimits, err := coretraefik.UploadLimits(coretraefik.UploadLimitsOptions{AppService: appService})
	if err != nil {
		panic(err)
	}
	devMode, err := coredevmode.Component(coredevmode.Options{
		AppService: appService,
		Volumes: []string{
			"./plugins/blocks:/var/www/ojs/plugins/blocks:z,rw",
			"./plugins/gateways:/var/www/ojs/plugins/gateways:z,rw",
			"./plugins/generic:/var/www/ojs/plugins/generic:z,rw",
			"./plugins/importexport:/var/www/ojs/plugins/importexport:z,rw",
			"./plugins/metadata:/var/www/ojs/plugins/metadata:z,rw",
			"./plugins/oaiMetadataFormats:/var/www/ojs/plugins/oaiMetadataFormats:z,rw",
			"./plugins/paymethod:/var/www/ojs/plugins/paymethod:z,rw",
			"./plugins/pubIds:/var/www/ojs/plugins/pubIds:z,rw",
			"./plugins/reports:/var/www/ojs/plugins/reports:z,rw",
			"./plugins/themes:/var/www/ojs/plugins/themes:z,rw",
		},
	})
	if err != nil {
		panic(err)
	}
	s.RegisterServiceComponents(plugin.ServiceComponentRegistryOptions{
		DisplayName: displayName,
		Components:  []corecomponent.ComposeServiceComponent{reverseProxy, uploadLimits, devMode},
	})
}
