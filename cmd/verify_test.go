package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	sitevalidate "github.com/libops/sitectl/pkg/validate"
)

type fakeOJSVerifyRuntime struct {
	run func([]string) (string, error)
}

func (f fakeOJSVerifyRuntime) ExecCapture(_ context.Context, argv []string) (string, error) {
	return f.run(argv)
}

func TestOJSVerifyChecksApplicationBehavior(t *testing.T) {
	t.Parallel()

	var calls []string
	runtime := fakeOJSVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "tools/upgrade.php check"):
			return "Code version: 3.5.0.5\nDatabase version: 3.5.0.5\nLatest version: 3.5.0.5", nil
		case strings.Contains(joined, ojsDatabaseProbePath):
			return "ojs@%\tjournal", nil
		case strings.Contains(joined, ojsConfigProbePath):
			return `{"installed":"1","base_url":"https://journal.example.org","files_dir":"/var/www/files","public_files_dir":"public","repository_id":"journal.example.org","task_runner":"1","smtp":"1","smtp_server":"mail","smtp_port":"25"}`, nil
		case strings.Contains(joined, "/index.php/journal/oai?verb=Identify"):
			return `<?xml version="1.0"?><OAI-PMH xmlns="http://www.openarchives.org/OAI/2.0/"><Identify/></OAI-PMH>`, nil
		case strings.Contains(joined, ojsStorageProbePath):
			return "storage writable", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runOJSVerifyChecks(context.Background(), runtime, false)
	assertAllOJSVerifyOK(t, results, 5)
	if !strings.Contains(strings.Join(calls, "\n"), "/index.php/journal/oai?verb=Identify") {
		t.Fatalf("enabled journal did not receive an OAI Identify request: %v", calls)
	}
}

func TestOJSVerifyNoJournalIsExplicitlyNotApplicable(t *testing.T) {
	t.Parallel()

	var oaiCalled bool
	runtime := fakeOJSVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "tools/upgrade.php check"):
			return "Code version: 3.5.0.5\nDatabase version: 3.5.0.5", nil
		case strings.Contains(joined, ojsDatabaseProbePath):
			return "ojs@%\t", nil
		case strings.Contains(joined, ojsConfigProbePath):
			return `{"installed":"1","base_url":"http://localhost","files_dir":"/var/www/files","public_files_dir":"public","repository_id":"localhost","task_runner":"1","smtp":"1","smtp_server":"mail","smtp_port":"25"}`, nil
		case strings.Contains(joined, "oai?verb=Identify"):
			oaiCalled = true
			return "", nil
		case strings.Contains(joined, ojsStorageProbePath):
			return "storage writable", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runOJSVerifyChecks(context.Background(), runtime, false)
	assertAllOJSVerifyOK(t, results, 5)
	if oaiCalled {
		t.Fatal("OAI request ran without an enabled journal")
	}
	if !strings.Contains(results[3].Detail, "no enabled journal") {
		t.Fatalf("OAI not-applicable evidence missing: %+v", results[3])
	}
}

func TestOJSVerifyDisposableModeUsesReversibleStorageProbe(t *testing.T) {
	t.Parallel()

	var storageCommand string
	runtime := fakeOJSVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "tools/upgrade.php check"):
			return "Code version: 3.5.0.5\nDatabase version: 3.5.0.5", nil
		case strings.Contains(joined, ojsDatabaseProbePath):
			return "ojs@%\t", nil
		case strings.Contains(joined, ojsConfigProbePath):
			return `{"installed":"1","base_url":"http://localhost","files_dir":"/var/www/files","public_files_dir":"public","repository_id":"localhost","task_runner":"1","smtp":"1","smtp_server":"mail","smtp_port":"25"}`, nil
		case strings.Contains(joined, ojsStorageProbePath) && strings.Contains(joined, "--disposable"):
			storageCommand = joined
			return "storage round trip complete", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runOJSVerifyChecks(context.Background(), runtime, true)
	assertAllOJSVerifyOK(t, results, 5)
	for _, required := range []string{"s6-setuidgid nginx", ojsStorageProbePath, "--disposable"} {
		if !strings.Contains(storageCommand, required) {
			t.Fatalf("disposable storage probe missing %q: %s", required, storageCommand)
		}
	}
}

func TestOJSVerifyRejectsRootDatabaseIdentity(t *testing.T) {
	t.Parallel()

	result := ojsDatabaseResult("root@localhost\tjournal", nil)
	if result.Status != sitevalidate.StatusFailed || !strings.Contains(result.Detail, "root") {
		t.Fatalf("root database identity was not rejected: %+v", result)
	}
}

func TestOJSVerifyUsesCheckedInPrograms(t *testing.T) {
	t.Parallel()

	for name, path := range map[string]string{
		"database": ojsDatabaseProbePath,
		"config":   ojsConfigProbePath,
		"storage":  ojsStorageProbePath,
	} {
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\n") {
			t.Fatalf("%s probe must be invoked by stable absolute path: %q", name, path)
		}
	}
}

func TestOJSVerifyRejectsIncompleteUpgradeAndConfigOutput(t *testing.T) {
	t.Parallel()

	if result := ojsVersionResult("Code version: 3.5.0.5", nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("incomplete upgrade output was accepted: %+v", result)
	}
	if result := ojsConfigResult(`{"installed":"1"}`, nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("incomplete runtime config was accepted: %+v", result)
	}
}

func TestOJSVerifyRejectsCredentialedCanonicalURL(t *testing.T) {
	t.Parallel()

	result := ojsConfigResult(`{"installed":"1","base_url":"https://user:password@journal.example.org","files_dir":"/var/www/files","public_files_dir":"public","repository_id":"journal.example.org","task_runner":"1","smtp":"1","smtp_server":"mail","smtp_port":"25"}`, nil)
	if result.Status != sitevalidate.StatusFailed {
		t.Fatalf("credentialed canonical URL was accepted: %+v", result)
	}
}

func assertAllOJSVerifyOK(t *testing.T, results []sitevalidate.Result, want int) {
	t.Helper()
	if len(results) != want {
		t.Fatalf("verification results = %d, want %d: %+v", len(results), want, results)
	}
	for _, result := range results {
		if result.Status != sitevalidate.StatusOK {
			t.Fatalf("verification result is not OK: %+v", result)
		}
	}
}
