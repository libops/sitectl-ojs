package cmd

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/docker"
	"github.com/libops/sitectl/pkg/plugin"
	sitevalidate "github.com/libops/sitectl/pkg/validate"
	"github.com/spf13/cobra"
)

const (
	ojsExpectedVersion  = "3.5.0.5"
	ojsDatabaseProbe    = `. /usr/local/share/libops/database.sh; database_mariadb_with_password "$DB_PASSWORD" --host="$DB_HOST" --port="$DB_PORT" --user="$DB_USER" --database="$DB_NAME" --batch --skip-column-names --execute="SELECT CURRENT_USER(), COALESCE((SELECT path FROM journals WHERE enabled = 1 ORDER BY journal_id LIMIT 1), '');"`
	ojsConfigProbe      = `$c = parse_ini_file("config.inc.php", true); echo json_encode(["installed" => $c["general"]["installed"] ?? null, "base_url" => $c["general"]["base_url"] ?? null, "files_dir" => $c["files"]["files_dir"] ?? null, "public_files_dir" => $c["files"]["public_files_dir"] ?? null, "repository_id" => $c["oai"]["repository_id"] ?? null, "task_runner" => $c["schedule"]["task_runner"] ?? null, "smtp" => $c["email"]["smtp"] ?? null, "smtp_server" => $c["email"]["smtp_server"] ?? null, "smtp_port" => $c["email"]["smtp_port"] ?? null], JSON_THROW_ON_ERROR);`
	ojsReadOnlyStorage  = `test -r /var/www/files && test -w /var/www/files && test -r /var/www/ojs/public && test -w /var/www/ojs/public && printf '%s\n' 'storage writable'`
	ojsStorageRoundTrip = `private=/var/www/files/.sitectl-verify-$$; public=/var/www/ojs/public/.sitectl-verify-$$; cleanup() { rm -f -- "$private" "$public"; }; trap cleanup EXIT INT TERM; printf '%s' sitectl-verify >"$private"; printf '%s' sitectl-verify >"$public"; test "$(cat "$private")" = sitectl-verify; test "$(cat "$public")" = sitectl-verify; cleanup; trap - EXIT INT TERM; printf '%s\n' 'storage round trip complete'`
)

var ojsJournalPathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type ojsVerifyRuntime interface {
	ExecCapture(context.Context, []string) (string, error)
}

type dockerOJSVerifyRuntime struct {
	client    *docker.DockerClient
	container string
}

func (r dockerOJSVerifyRuntime) ExecCapture(ctx context.Context, argv []string) (string, error) {
	return docker.ExecCapture(ctx, r.client, r.container, ojsRoot, argv)
}

type ojsVerifyRunner struct {
	sdk        *plugin.SDK
	disposable bool
}

func (r *ojsVerifyRunner) BindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&r.disposable, "disposable", false, "Write, read, and remove probe files in OJS storage. Use only for a disposable CI site, never a retained site.")
}

func (r *ojsVerifyRunner) Run(cmd *cobra.Command, _ *config.Context) ([]sitevalidate.Result, error) {
	if r.sdk == nil {
		return nil, fmt.Errorf("OJS verifier SDK is not initialized")
	}
	verifyContext, err := r.sdk.GetContext()
	if err != nil {
		return nil, err
	}
	client, err := r.sdk.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("connect to Docker for OJS verification: %w", err)
	}
	defer func() { _ = client.Close() }()
	container, err := client.GetContainerNameContext(cmd.Context(), verifyContext, ojsService)
	if err != nil {
		return nil, fmt.Errorf("find running OJS container: %w", err)
	}
	return runOJSVerifyChecks(cmd.Context(), dockerOJSVerifyRuntime{client: client, container: container}, r.disposable), nil
}

func runOJSVerifyChecks(ctx context.Context, runtime ojsVerifyRuntime, disposable bool) []sitevalidate.Result {
	results := make([]sitevalidate.Result, 0, 5)

	upgradeOutput, upgradeErr := runtime.ExecCapture(ctx, []string{"php", "tools/upgrade.php", "check"})
	results = append(results, ojsVersionResult(upgradeOutput, upgradeErr))

	databaseOutput, databaseErr := runtime.ExecCapture(ctx, []string{"bash", "-lc", ojsDatabaseProbe})
	results = append(results, ojsDatabaseResult(databaseOutput, databaseErr))

	configOutput, configErr := runtime.ExecCapture(ctx, []string{"php", "-r", ojsConfigProbe})
	results = append(results, ojsConfigResult(configOutput, configErr))

	journalPath := ojsJournalPath(databaseOutput)
	switch {
	case databaseErr != nil:
		results = append(results, ojsVerifyFailed("verify:ojs:oai", "enabled journal discovery failed with the database probe", "repair the scoped OJS database connection"))
	case journalPath == "":
		results = append(results, ojsVerifyOK("verify:ojs:oai", "no enabled journal; OAI publication is not yet applicable"))
	case !ojsJournalPathPattern.MatchString(journalPath):
		results = append(results, ojsVerifyFailed("verify:ojs:oai", fmt.Sprintf("enabled journal returned unsafe path %q", journalPath), "repair the journal path before exposing OAI-PMH"))
	default:
		oaiURL := "http://127.0.0.1/index.php/" + url.PathEscape(journalPath) + "/oai?verb=Identify"
		oaiOutput, oaiErr := runtime.ExecCapture(ctx, []string{"curl", "--connect-timeout", "2", "--max-time", "30", "-fsS", "-H", "Accept: application/xml", oaiURL})
		results = append(results, ojsOAIResult(oaiOutput, oaiErr, journalPath))
	}

	storageScript := ojsReadOnlyStorage
	storageDetail := "private and public storage are writable by the OJS service account"
	if disposable {
		storageScript = ojsStorageRoundTrip
		storageDetail = "private and public storage completed a reversible write/read/delete round trip"
	}
	_, storageErr := runtime.ExecCapture(ctx, []string{"s6-setuidgid", "nginx", "sh", "-ec", storageScript})
	if storageErr != nil {
		results = append(results, ojsVerifyFailed("verify:ojs:storage", storageErr.Error(), "repair ownership and permissions for /var/www/files and /var/www/ojs/public"))
	} else {
		results = append(results, ojsVerifyOK("verify:ojs:storage", storageDetail))
	}

	return results
}

func ojsVersionResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return ojsVerifyFailed("verify:ojs:version-schema", commandErr.Error(), "run the supported OJS upgrade and inspect its output")
	}
	codeVersion := ojsVersionLine(output, "Code version:")
	databaseVersion := ojsVersionLine(output, "Database version:")
	if codeVersion == "" || databaseVersion == "" {
		return ojsVerifyFailed("verify:ojs:version-schema", "upgrade check omitted code or database version", "confirm tools/upgrade.php matches the supported OJS release")
	}
	if codeVersion != ojsExpectedVersion {
		return ojsVerifyFailed("verify:ojs:version-schema", fmt.Sprintf("running code version is %s, expected %s", codeVersion, ojsExpectedVersion), "rebuild from the plugin's supported OJS base image")
	}
	if databaseVersion != codeVersion {
		return ojsVerifyFailed("verify:ojs:version-schema", fmt.Sprintf("database version %s differs from code version %s", databaseVersion, codeVersion), "back up the site and run the supported OJS upgrade")
	}
	return ojsVerifyOK("verify:ojs:version-schema", fmt.Sprintf("code and database are both %s", codeVersion))
}

func ojsVersionLine(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= len(prefix) && strings.EqualFold(trimmed[:len(prefix)], prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return ""
}

func ojsDatabaseResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return ojsVerifyFailed("verify:ojs:database-identity", commandErr.Error(), "check the scoped OJS database secret and MariaDB connectivity")
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return ojsVerifyFailed("verify:ojs:database-identity", "database returned no current user", "check the scoped OJS database secret")
	}
	username, _, _ := strings.Cut(fields[0], "@")
	if strings.EqualFold(username, "root") {
		return ojsVerifyFailed("verify:ojs:database-identity", "OJS is connected as the MariaDB root user", "configure OJS with its scoped application database user")
	}
	return ojsVerifyOK("verify:ojs:database-identity", fields[0])
}

func ojsJournalPath(databaseOutput string) string {
	fields := strings.Fields(databaseOutput)
	if len(fields) < 2 {
		return ""
	}
	return fields[1]
}

func ojsConfigResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return ojsVerifyFailed("verify:ojs:runtime-config", commandErr.Error(), "inspect the rendered OJS config.inc.php")
	}
	var probe struct {
		Installed      string `json:"installed"`
		BaseURL        string `json:"base_url"`
		FilesDir       string `json:"files_dir"`
		PublicFilesDir string `json:"public_files_dir"`
		RepositoryID   string `json:"repository_id"`
		TaskRunner     string `json:"task_runner"`
		SMTP           string `json:"smtp"`
		SMTPServer     string `json:"smtp_server"`
		SMTPPort       string `json:"smtp_port"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &probe); err != nil {
		return ojsVerifyFailed("verify:ojs:runtime-config", fmt.Sprintf("decode config probe: %v", err), "inspect the rendered OJS config.inc.php")
	}
	if !ojsINIEnabled(probe.Installed) {
		return ojsVerifyFailed("verify:ojs:runtime-config", "OJS is not marked installed", "wait for setup to finish and inspect OJS setup logs")
	}
	parsedBaseURL, err := url.ParseRequestURI(probe.BaseURL)
	if err != nil || parsedBaseURL.Host == "" || (parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https") || parsedBaseURL.User != nil {
		return ojsVerifyFailed("verify:ojs:runtime-config", fmt.Sprintf("invalid base_url %q", probe.BaseURL), "reconcile ingress so OJS receives one canonical HTTP or HTTPS URL")
	}
	if probe.FilesDir != "/var/www/files" || probe.PublicFilesDir != "public" {
		return ojsVerifyFailed("verify:ojs:runtime-config", fmt.Sprintf("unexpected file roots private=%q public=%q", probe.FilesDir, probe.PublicFilesDir), "restore the managed private and public storage paths")
	}
	if strings.TrimSpace(probe.RepositoryID) == "" {
		return ojsVerifyFailed("verify:ojs:runtime-config", "OAI repository_id is empty", "reconcile ingress to set OJS_OAI_REPOSITORY_ID")
	}
	if !ojsINIEnabled(probe.TaskRunner) {
		return ojsVerifyFailed("verify:ojs:runtime-config", "the built-in scheduled task runner is disabled", "enable one authoritative scheduler before relying on recurring work")
	}
	if !ojsINIEnabled(probe.SMTP) || strings.TrimSpace(probe.SMTPServer) == "" || strings.TrimSpace(probe.SMTPPort) == "" {
		return ojsVerifyFailed("verify:ojs:runtime-config", "SMTP transport is incomplete", "configure the managed SMTP relay host and port")
	}
	return ojsVerifyOK("verify:ojs:runtime-config", fmt.Sprintf("base URL %s; OAI repository %s; scheduler and SMTP configured", probe.BaseURL, probe.RepositoryID))
}

func ojsINIEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "true", "yes":
		return true
	default:
		return false
	}
}

func ojsOAIResult(output string, commandErr error, journalPath string) sitevalidate.Result {
	if commandErr != nil {
		return ojsVerifyFailed("verify:ojs:oai", commandErr.Error(), "confirm the enabled journal exposes a working OAI-PMH endpoint")
	}
	var envelope struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(strings.TrimSpace(output)), &envelope); err != nil {
		return ojsVerifyFailed("verify:ojs:oai", fmt.Sprintf("decode OAI response: %v", err), "inspect the journal OAI-PMH route")
	}
	if envelope.XMLName.Local != "OAI-PMH" {
		return ojsVerifyFailed("verify:ojs:oai", fmt.Sprintf("unexpected OAI root element %q", envelope.XMLName.Local), "inspect the journal OAI-PMH route")
	}
	return ojsVerifyOK("verify:ojs:oai", fmt.Sprintf("Identify returned an OAI-PMH envelope for journal %s", journalPath))
}

func ojsVerifyOK(name, detail string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusOK, Detail: detail}
}

func ojsVerifyFailed(name, detail, fix string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusFailed, Detail: detail, FixHint: fix}
}

var _ plugin.VerifyRunner = (*ojsVerifyRunner)(nil)
