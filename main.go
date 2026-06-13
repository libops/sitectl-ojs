package main

import (
	"fmt"

	"github.com/libops/sitectl-ojs/cmd"
	"github.com/libops/sitectl/pkg/plugin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sdk := plugin.NewSDK(plugin.Metadata{
		Name:         "ojs",
		Version:      fmt.Sprintf("%s (Built on %s from Git SHA %s)", version, date, commit),
		Description:  "Open Journal Systems helpers",
		Author:       "libops",
		TemplateRepo: "https://github.com/libops/ojs",
		Includes:     cmd.IncludedPlugins(),
	})

	cmd.RegisterCommands(sdk)
	sdk.Execute()
}
