package cmd

import (
	"fmt"
	"strings"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
	"github.com/spf13/cobra"
)

const (
	ojsService = "ojs"
	ojsRoot    = "/var/www/ojs"
)

func registerOJSCommands(s *sitectlplugin.SDK) {
	s.AddCommand(ojsToolCommand(s))
	s.AddCommand(ojsPKPToolCommand(s))
	s.AddCommand(ojsUpgradeCommand(s))
	s.AddCommand(ojsImportExportCommand(s))
	s.AddCommand(ojsRebuildSearchIndexCommand(s))
	s.AddCommand(ojsScheduledTasksCommand(s))
	s.AddCommand(ojsJobsCommand(s))
	s.AddCommand(ojsPluginsCommand(s))
}

func ojsToolCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "tool TOOL [args...]",
		Short:              "Run an OJS PHP tool from /var/www/ojs/tools",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := normalizePHPToolName(args[0])
			if err != nil {
				return err
			}
			toolArgs := []string{"php", ojsRoot + "/tools/" + tool}
			toolArgs = append(toolArgs, args[1:]...)
			return runOJSExec(s, cmd, toolArgs...)
		},
	}
}

func ojsPKPToolCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "pkp-tool TOOL [args...]",
		Short:              "Run a PKP PHP tool from /var/www/ojs/lib/pkp/tools",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := normalizePHPToolName(args[0])
			if err != nil {
				return err
			}
			toolArgs := []string{"php", ojsRoot + "/lib/pkp/tools/" + tool}
			toolArgs = append(toolArgs, args[1:]...)
			return runOJSExec(s, cmd, toolArgs...)
		},
	}
}

func ojsUpgradeCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "upgrade [upgrade.php args...]",
		Short:              "Run the OJS database/code upgrade tool",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"upgrade"}
			}
			return runOJSPHPTool(s, cmd, "tools/upgrade.php", args...)
		},
	}
}

func ojsImportExportCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "import-export [importExport.php args...]",
		Short:              "Run the OJS import/export tool",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOJSPHPTool(s, cmd, "tools/importExport.php", args...)
		},
	}
}

func ojsRebuildSearchIndexCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "rebuild-search-index [rebuildSearchIndex.php args...]",
		Short:              "Rebuild the OJS search index",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOJSPHPTool(s, cmd, "tools/rebuildSearchIndex.php", args...)
		},
	}
}

func ojsScheduledTasksCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "scheduled-tasks [scheduler.php args...]",
		Short:              "Run OJS/PKP scheduled tasks",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"run"}
			}
			return runOJSPHPTool(s, cmd, "lib/pkp/tools/scheduler.php", args...)
		},
	}
}

func ojsJobsCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "jobs [jobs.php args...]",
		Short:              "Run OJS/PKP queued job helpers",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"run"}
			}
			return runOJSPHPTool(s, cmd, "lib/pkp/tools/jobs.php", args...)
		},
	}
}

func ojsPluginsCommand(s *sitectlplugin.SDK) *cobra.Command {
	return &cobra.Command{
		Use:                "plugins [plugins.php args...]",
		Short:              "Run OJS/PKP plugin maintenance helpers",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOJSPHPTool(s, cmd, "lib/pkp/tools/plugins.php", args...)
		},
	}
}

func runOJSPHPTool(s *sitectlplugin.SDK, cmd *cobra.Command, path string, args ...string) error {
	toolArgs := []string{"php", ojsRoot + "/" + path}
	toolArgs = append(toolArgs, args...)
	return runOJSExec(s, cmd, toolArgs...)
}

func runOJSExec(s *sitectlplugin.SDK, cmd *cobra.Command, args ...string) error {
	return s.RunActiveComposeProjectCommand(cmd, sitectlplugin.DockerComposeExecCommand(ojsService, args...))
}

func normalizePHPToolName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("tool name must be a PHP file name, not a path")
	}
	if !strings.HasSuffix(name, ".php") {
		name += ".php"
	}
	return name, nil
}
