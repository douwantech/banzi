package main

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Keeps the manual whole: every command has a name, group, index line, help and a run.
func TestCommandsComplete(t *testing.T) {
	registerAll()
	if len(commands) == 0 {
		t.Fatal("no commands registered")
	}
	seen := map[string]bool{}
	for _, c := range commands {
		if c.name == "" || c.group == "" || c.one == "" || c.help == "" {
			t.Errorf("command %q missing name/group/one/help", c.name)
		}
		if c.run == nil {
			t.Errorf("command %q has no run", c.name)
		}
		if seen[c.name] {
			t.Errorf("duplicate command %q", c.name)
		}
		seen[c.name] = true
	}
}

// record replaces execAsc for one test: it keeps every asc call and answers with the
// given exit codes in order (0 once they run out).
func record(t *testing.T, exits ...int) *[][]string {
	t.Helper()
	calls := &[][]string{}
	real := execAsc
	execAsc = func(args []string) int {
		*calls = append(*calls, append([]string{}, args...))
		if n := len(*calls); n <= len(exits) {
			return exits[n-1]
		}
		return 0
	}
	t.Cleanup(func() { execAsc = real })
	registerAll()
	return calls
}

func run(t *testing.T, name string, args ...string) int {
	t.Helper()
	c := get(name)
	if c == nil {
		t.Fatalf("no command %q", name)
	}
	return c.run(args)
}

func words(s string) []string { return strings.Fields(s) }

func TestBuildArchivesThenExportsAnIpa(t *testing.T) {
	calls := record(t)
	rc := run(t, "build", "--workspace", "App.xcworkspace", "--scheme", "App", "--configuration", "Release", "--", "--signing-style", "manual")
	want := [][]string{
		words("xcode archive --archive-path build/App.xcarchive --overwrite --workspace App.xcworkspace --scheme App --configuration Release"),
		words("xcode export --archive-path build/App.xcarchive --ipa-path build/App.ipa --overwrite --signing-style manual"),
	}
	if rc != 0 || !reflect.DeepEqual(*calls, want) {
		t.Errorf("rc=%d calls=%v\nwant %v", rc, *calls, want)
	}
}

func TestBuildDoesNotExportWhenTheArchiveFails(t *testing.T) {
	calls := record(t, 65)
	if rc := run(t, "build", "--scheme=App"); rc != 65 || len(*calls) != 1 {
		t.Errorf("rc=%d calls=%v, want the archive's exit code and no export", rc, *calls)
	}
}

func TestBuildNeedsASchemeAndDryRunsNothing(t *testing.T) {
	calls := record(t)
	if rc := run(t, "build", "--configuration", "Release"); rc != 2 {
		t.Errorf("without --scheme: rc=%d, want 2", rc)
	}
	if rc := run(t, "build", "--scheme", "App", "--dry"); rc != 0 {
		t.Errorf("dry run: rc=%d, want 0", rc)
	}
	if len(*calls) != 0 {
		t.Errorf("asc was run: %v", *calls)
	}
}

func TestBumpEditsTheProjectVersion(t *testing.T) {
	calls := record(t)
	if rc := run(t, "bump", "1.2.3", "48", "--project", "ios/App.xcodeproj"); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := [][]string{words("xcode version edit --version 1.2.3 --build-number 48 --project ios/App.xcodeproj")}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
	if rc := run(t, "bump", "1.2.3"); rc != 2 || len(*calls) != 1 {
		t.Errorf("one argument: rc=%d calls=%d, want a usage error and no asc call", rc, len(*calls))
	}
}

func TestBumpNextTakesTheBuildNumberFromAppStoreConnect(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	calls := record(t)
	if rc := run(t, "bump", "--next"); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := [][]string{words("xcode version edit --next-build-number --app 123456789")}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
}

func TestBetaOnlyUploadsUnlessAGroupIsNamed(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	calls := record(t)
	run(t, "beta", "build/App.ipa")
	run(t, "beta", "build/App.ipa", "--group", "Internal")
	want := [][]string{
		words("publish testflight --app 123456789 --ipa build/App.ipa --upload-only"),
		words("publish testflight --app 123456789 --ipa build/App.ipa --group Internal"),
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
}

func TestCheckValidatesWithoutUploading(t *testing.T) {
	calls := record(t)
	run(t, "check", "build/App.ipa")
	want := [][]string{words("xcode validate --ipa build/App.ipa")}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
}

func TestReleaseAttachesTheBuildAndDoesNotSubmitByItself(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	calls := record(t)
	run(t, "release", "build/App.ipa", "1.2.3")
	want := [][]string{words("publish appstore --app 123456789 --ipa build/App.ipa --version 1.2.3")}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
}

func TestNotarizeSubmitsThenStaplesAndStopsWhenRejected(t *testing.T) {
	calls := record(t)
	run(t, "notarize", "dist/App.pkg")
	want := [][]string{
		words("notarization submit --file dist/App.pkg --wait"),
		words("notarization staple --file dist/App.pkg --confirm"),
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
	rejected := record(t, 1)
	if rc := run(t, "notarize", "dist/App.pkg"); rc != 1 || len(*rejected) != 1 {
		t.Errorf("rejected: rc=%d calls=%v, want no staple", rc, *rejected)
	}
}

func TestDoctorChecksTheToolchainAndTheAccount(t *testing.T) {
	if _, err := exec.LookPath("asc"); err != nil {
		t.Skip("doctor looks for asc before running it")
	}
	calls := record(t)
	run(t, "doctor")
	want := [][]string{words("xcode doctor"), words("account status")}
	if !reflect.DeepEqual(*calls, want) {
		t.Errorf("calls=%v\nwant %v", *calls, want)
	}
}

// The credentials are read by asc, so the manual has to name the variables asc knows.
func TestManualNamesTheCredentialVariablesAscReads(t *testing.T) {
	registerAll()
	all := manual()
	if strings.Contains(all, "ASC_KEY_P8") {
		t.Error("the manual names ASC_KEY_P8, which asc does not read")
	}
	for _, name := range []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY"} {
		if !strings.Contains(all, name) {
			t.Errorf("the manual does not name %s", name)
		}
	}
}

// With asc installed, every asc command ship passes through must exist there: this is what
// catches a subcommand that was renamed or never did what ship's help says. Skipped without asc.
func TestEveryAscCommandExists(t *testing.T) {
	if _, err := exec.LookPath("asc"); err != nil {
		t.Skip("asc is not installed")
	}
	for _, path := range []string{
		"xcode doctor", "account status", "xcode archive", "xcode export", "xcode version edit", "signing",
		"screenshots", "xcode validate", "publish testflight", "publish appstore", "notarization submit", "notarization staple",
	} {
		if out, err := exec.Command("asc", append(words(path), "--help")...).CombinedOutput(); err != nil {
			t.Errorf("asc %s --help: %v\n%.200s", path, err, out)
		}
	}
}
