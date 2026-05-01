package etch

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return string(out)
}

func testGitMayFail(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Run()
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testGit(t, dir, "init", "-b", "main")
	testGit(t, dir, "config", "user.name", "Brandon Bloom")
	testGit(t, dir, "config", "user.email", "brandon@example.com")
	return dir
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-m", msg)
	return stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
}

func runCLIInDir(t *testing.T, dir string, args ...string) (bytes.Buffer, bytes.Buffer, exitCode, error) {
	t.Helper()
	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, args, &out, &errb)
	return out, errb, code, err
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// setProcessCWDForTest exists for tests whose subject is process-CWD behavior.
// Prefer runCLIAt/OpenWorkspaceAt in ordinary repository tests.
func setProcessCWDForTest(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}
