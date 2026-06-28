package cmd

import "github.com/libops/sitectl/pkg/plugin"

var ojsHealthcheckRunner = plugin.StandardComposeWebHealthcheck(plugin.StandardComposeWebHealthcheckOptions{
	AppService:              "ojs",
	HTTPName:                "http:ojs",
	DefaultScheme:           "http",
	DefaultDomain:           "localhost",
	DatabaseService:         "mariadb",
	CheckDatabaseDependency: true,
})
