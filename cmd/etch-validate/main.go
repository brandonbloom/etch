package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type result struct {
	Workflow       string `json:"workflow"`
	Task           string `json:"task"`
	Success        bool   `json:"success"`
	ExpectedTree   bool   `json:"expected_tree_match"`
	IntendedCommit bool   `json:"intended_commit_match"`
	Recovery       string `json:"recovery_outcome,omitempty"`
	ToolCalls      int    `json:"tool_calls"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	StdoutBytes    int    `json:"stdout_bytes"`
	StderrBytes    int    `json:"stderr_bytes"`
	Error          string `json:"error,omitempty"`
}

func main() {
	etchPath := flag.String("etch", "", "path to etch binary; built from ./cmd/etch when empty")
	flag.Parse()
	bin := *etchPath
	var cleanup func()
	var err error
	if bin == "" {
		bin, cleanup, err = buildEtch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "etch-validate: %v\n", err)
			os.Exit(1)
		}
	}
	if cleanup != nil {
		defer cleanup()
	}
	results := []result{
		runJSONSetBaseline(),
		runJSONSetEtch(bin),
		runRunScriptEtch(bin),
		runPlanDryRunEtch(bin),
		runDirtyRecoveryEtch(bin),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "etch-validate: %v\n", err)
		os.Exit(1)
	}
	for _, r := range results {
		if !r.Success {
			os.Exit(1)
		}
	}
}

func buildEtch() (string, func(), error) {
	dir, err := os.MkdirTemp("", "etch-validate-*")
	if err != nil {
		return "", nil, err
	}
	bin := filepath.Join(dir, "etch")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/etch")
	cmd.Env = append(os.Environ(), "GOCACHE=/private/tmp/etch-gocache", "GOMODCACHE=/private/tmp/etch-gomodcache")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("build etch: %v: %s", err, stderr.String())
	}
	return bin, func() { _ = os.RemoveAll(dir) }, nil
}

func runJSONSetBaseline() (res result) {
	dir := fixtureRepo()
	start := time.Now()
	toolCalls := 0
	res = result{Workflow: "baseline-go-edit", Task: "json-set"}
	defer func() { finish(&res, start, toolCalls) }()
	mustWrite(filepath.Join(dir, "state.json"), "{\n  \"status\": \"complete\"\n}\n")
	toolCalls++
	run(dir, "git", "add", "state.json")
	toolCalls++
	out, errb, err := run(dir, "git", "commit", "-m", "baseline set state status")
	toolCalls++
	res.StdoutBytes += len(out)
	res.StderrBytes += len(errb)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.ExpectedTree = containsGitShow(dir, "HEAD:state.json", `"status": "complete"`)
	res.IntendedCommit = strings.Contains(gitOut(dir, "log", "-1", "--format=%s"), "baseline set")
	res.Success = res.ExpectedTree && res.IntendedCommit
	return
}

func runJSONSetEtch(bin string) (res result) {
	dir := fixtureRepo()
	start := time.Now()
	toolCalls := 0
	res = result{Workflow: "etch-one-shot", Task: "json-set"}
	defer func() { finish(&res, start, toolCalls) }()
	out, errb, err := run(dir, bin, "set", "state.json", "status", "complete")
	toolCalls++
	res.StdoutBytes += len(out)
	res.StderrBytes += len(errb)
	if err != nil {
		res.Error = err.Error() + ": " + string(errb)
		return
	}
	res.ExpectedTree = containsGitShow(dir, "HEAD:state.json", `"status": "complete"`)
	res.IntendedCommit = strings.Contains(gitOut(dir, "log", "-1", "--format=%s"), "etch set state.json $.status")
	res.Success = res.ExpectedTree && res.IntendedCommit
	return
}

func runRunScriptEtch(bin string) (res result) {
	dir := fixtureRepo()
	start := time.Now()
	toolCalls := 0
	res = result{Workflow: "etch-run", Task: "multi-op"}
	defer func() { finish(&res, start, toolCalls) }()
	script := filepath.Join(dir, "ops.etch")
	mustWrite(script, "set state.json status complete\nset state.json owner Brandon\n")
	out, errb, err := run(dir, bin, "run", script)
	toolCalls++
	res.StdoutBytes += len(out)
	res.StderrBytes += len(errb)
	if err != nil {
		res.Error = err.Error() + ": " + string(errb)
		return
	}
	head := gitOut(dir, "show", "HEAD:state.json")
	res.ExpectedTree = strings.Contains(head, `"status": "complete"`) && strings.Contains(head, `"owner": "Brandon"`)
	res.IntendedCommit = strings.Contains(gitOut(dir, "log", "-1", "--format=%s"), "etch: 2 changes")
	res.Success = res.ExpectedTree && res.IntendedCommit
	return
}

func runPlanDryRunEtch(bin string) (res result) {
	dir := fixtureRepo()
	start := time.Now()
	toolCalls := 0
	res = result{Workflow: "etch-plan-dry-run", Task: "json-set-review"}
	defer func() { finish(&res, start, toolCalls) }()
	out, errb, err := run(dir, bin, "--plan", "set", "state.json", "status", "complete")
	toolCalls++
	res.StdoutBytes += len(out)
	res.StderrBytes += len(errb)
	if err != nil || !bytes.Contains(out, []byte(`"tree"`)) {
		res.Error = fmt.Sprintf("plan failed: %v %s", err, errb)
		return
	}
	patch, patchErr, err := run(dir, bin, "--dry-run", "set", "state.json", "status", "complete")
	toolCalls++
	res.StdoutBytes += len(patch)
	res.StderrBytes += len(patchErr)
	if err != nil {
		res.Error = err.Error() + ": " + string(patchErr)
		return
	}
	patchPath := filepath.Join(dir, "plan.patch")
	mustWrite(patchPath, string(patch))
	_, amErr, err := run(dir, "git", "am", patchPath)
	toolCalls++
	res.StderrBytes += len(amErr)
	if err != nil {
		res.Error = err.Error() + ": " + string(amErr)
		return
	}
	res.ExpectedTree = containsGitShow(dir, "HEAD:state.json", `"status": "complete"`)
	res.IntendedCommit = strings.Contains(gitOut(dir, "log", "-1", "--format=%s"), "etch set state.json $.status")
	res.Success = res.ExpectedTree && res.IntendedCommit
	return
}

func runDirtyRecoveryEtch(bin string) (res result) {
	dir := fixtureRepo()
	mustWrite(filepath.Join(dir, "state.json"), `{"status":"dirty","note":"local"}`+"\n")
	start := time.Now()
	toolCalls := 0
	res = result{Workflow: "etch-recovery", Task: "dirty-worktree-conflict"}
	defer func() { finish(&res, start, toolCalls) }()
	_, errb, err := run(dir, bin, "set", "state.json", "status", "complete")
	toolCalls++
	res.StderrBytes += len(errb)
	head := gitOut(dir, "show", "HEAD:state.json")
	wt, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	res.ExpectedTree = strings.Contains(head, `"status": "complete"`)
	res.IntendedCommit = strings.Contains(gitOut(dir, "log", "-1", "--format=%s"), "etch set state.json $.status")
	res.Recovery = "conflict-markers"
	res.Success = err != nil && bytes.Contains(wt, []byte("<<<<<<<")) && res.ExpectedTree && res.IntendedCommit
	if !res.Success {
		res.Error = fmt.Sprintf("expected conflict recovery, err=%v stderr=%s wt=%s", err, errb, wt)
	}
	return
}

func finish(res *result, start time.Time, toolCalls int) {
	res.ElapsedMS = time.Since(start).Milliseconds()
	if res.ToolCalls == 0 {
		res.ToolCalls = toolCalls
	}
}

func fixtureRepo() string {
	dir, err := os.MkdirTemp("", "etch-validation-repo-*")
	must(err)
	runMust(dir, "git", "init", "-b", "main")
	runMust(dir, "git", "config", "user.name", "Brandon Bloom")
	runMust(dir, "git", "config", "user.email", "brandon@example.com")
	mustWrite(filepath.Join(dir, "state.json"), `{"status":"open","note":"base"}`+"\n")
	runMust(dir, "git", "add", ".")
	runMust(dir, "git", "commit", "-m", "initial")
	return dir
}

func run(dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func runMust(dir, name string, args ...string) {
	_, stderr, err := run(dir, name, args...)
	if err != nil {
		panic(fmt.Sprintf("%s %v: %v %s", name, args, err, stderr))
	}
}

func gitOut(dir string, args ...string) string {
	out, stderr, err := run(dir, "git", args...)
	if err != nil {
		panic(fmt.Sprintf("git %v: %v %s", args, err, stderr))
	}
	return string(out)
}

func containsGitShow(dir, spec, needle string) bool {
	return strings.Contains(gitOut(dir, "show", spec), needle)
}

func mustWrite(path, content string) {
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, []byte(content), 0o644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
