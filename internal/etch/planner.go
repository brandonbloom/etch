package etch

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/brandonbloom/etch/internal/jsonx"
	"github.com/lattice-substrate/json-canon/jcs"
)

type Executor struct {
	opts GlobalOptions
}

var beforeUpdateRefHook func(attempt int)
var retrySleep = time.Sleep
var retryRandomDuration = randomRetryDuration

func NewExecutor(opts GlobalOptions) Executor {
	return Executor{opts: opts}
}

func (e Executor) openWorkspace() (*Workspace, error) {
	if e.opts.CWD != "" {
		return OpenWorkspaceAt(e.opts.CWD, e.opts.Untracked)
	}
	return OpenWorkspace(e.opts.Untracked)
}

func (e Executor) Run(ops []Operation, stdout, stderr interface {
	Write([]byte) (int, error)
}) (exitCode, error) {
	if e.opts.AllowEmpty {
		hasMutating := false
		for _, op := range ops {
			if op.Class != ClassGuard {
				hasMutating = true
				break
			}
		}
		if !hasMutating {
			return exitUsage, usagef("--allow-empty requires at least one mutating command")
		}
	}

	var lastErr error
	attempts := e.opts.Retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			sleepRetry(attempt)
		}
		w, err := e.openWorkspace()
		if err != nil {
			return exitFailure, err
		}
		plan, err := PlanOperations(w, e.opts, ops)
		if err != nil {
			return classifyErr(err), err
		}
		if e.opts.Plan {
			return exitOK, jsonOut(stdout, plan)
		}
		if e.opts.DryRun {
			patch, err := RenderDryRun(w, plan)
			if err != nil {
				return exitFailure, err
			}
			_, _ = stdout.Write([]byte(patch))
			return exitOK, nil
		}
		if !plan.Mutating || (!plan.Changed && !e.opts.AllowEmpty) {
			_, _ = stderr.Write([]byte("etch: nothing to do\n"))
			return exitOK, nil
		}
		materializer, err := NewMaterializer(w, plan, e.opts.NoCheckout)
		if err != nil {
			return exitFailure, err
		}
		if err := materializer.Preflight(); err != nil {
			return classifyErr(err), err
		}
		tree, err := w.writePlannedTree(plan)
		if err != nil {
			return exitFailure, err
		}
		commit, err := w.writeCommitObject(tree, plan.Commit.Message)
		if err != nil {
			return exitFailure, err
		}
		if beforeUpdateRefHook != nil {
			beforeUpdateRefHook(attempt)
		}
		if err := w.updateRef(commit); err != nil {
			lastErr = err
			if isRetryableCAS(err) {
				continue
			}
			return exitFailure, err
		}
		if e.opts.NoCheckout {
			fmt.Fprintf(stderr, "etch: committed %.7s; checkout skipped by --no-checkout\n", commit)
			fmt.Fprintln(stderr, "etch: working tree and index were not updated for touched paths")
			return exitOK, nil
		}
		if err := materializer.Apply(commit, stderr); err != nil {
			return exitFailure, err
		}
		fmt.Fprintf(stderr, "etch: committed %.7s\n", commit)
		return exitOK, nil
	}
	if lastErr != nil {
		return exitFailure, failf("retry budget exhausted: %v", lastErr)
	}
	return exitFailure, failf("retry budget exhausted")
}

func classifyErr(err error) exitCode {
	var coded errWithCode
	if errors.As(err, &coded) {
		return coded.code
	}
	return exitFailure
}

func isRetryableCAS(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "cannot lock ref") || strings.Contains(msg, "is at") || strings.Contains(msg, "but expected")
}

func sleepRetry(attempt int) {
	window := retryBackoffWindow(attempt)
	if window.Max == 0 {
		return
	}
	retrySleep(retryRandomDuration(window))
}

type retryWindow struct {
	Min time.Duration
	Max time.Duration
}

func retryBackoffWindow(attempt int) retryWindow {
	switch attempt {
	case 1:
		return retryWindow{}
	case 2:
		return retryWindow{Min: 50 * time.Millisecond, Max: 150 * time.Millisecond}
	case 3:
		return retryWindow{Min: 100 * time.Millisecond, Max: 300 * time.Millisecond}
	case 4:
		return retryWindow{Min: 200 * time.Millisecond, Max: 600 * time.Millisecond}
	case 5:
		return retryWindow{Min: 400 * time.Millisecond, Max: 1200 * time.Millisecond}
	default:
		if attempt >= 6 {
			return retryWindow{Min: 800 * time.Millisecond, Max: 2000 * time.Millisecond}
		}
		return retryWindow{}
	}
}

func randomRetryDuration(window retryWindow) time.Duration {
	if window.Max <= window.Min {
		return window.Min
	}
	width := window.Max - window.Min
	n, err := rand.Int(rand.Reader, big.NewInt(int64(width)+1))
	if err != nil {
		return window.Min + width/2
	}
	return window.Min + time.Duration(n.Int64())
}

func PlanOperations(w *Workspace, opts GlobalOptions, ops []Operation) (*Plan, error) {
	files := map[string]fileChange{}
	planOps := make([]Operation, 0, len(ops))
	mutating := false

	for _, op := range ops {
		planned, _, err := planOne(w, files, op)
		if err != nil {
			if op.Loc.Name != "" {
				return nil, errWithCode{code: classifyErr(err), err: fmt.Errorf("%s%s", op.Loc.Prefix(), err)}
			}
			return nil, err
		}
		if planned.Class != ClassGuard {
			mutating = true
		}
		planOps = append(planOps, planned)
	}

	pruneUnchangedFiles(files)
	changed := fileChangesChanged(files)
	tree, err := w.computePlannedTreeOID(files)
	if err != nil {
		return nil, err
	}
	msg := buildCommitMessage(opts, planOps)
	planFiles := map[string]PlanFile{}
	for _, k := range sortedKeys(files) {
		ch := files[k]
		beforeHash := shaHex(ch.Before)
		if ch.AbsentBefore {
			beforeHash = shaHex(nil)
		}
		afterHash := shaHex(ch.After)
		if ch.AbsentAfter {
			afterHash = shaHex(nil)
		}
		planFiles[ch.Path] = PlanFile{BeforeSHA256: beforeHash, AfterSHA256: afterHash}
	}
	plan := &Plan{
		Schema:     planSchema,
		Ref:        w.Ref,
		BaseCommit: w.Head,
		Operations: planOps,
		Files:      planFiles,
		Tree:       tree,
		Commit:     PlanCommit{Message: msg},
		Touched:    files,
		Mutating:   mutating,
		Changed:    changed,
	}
	if w.Unborn {
		plan.BaseCommit = ""
	}
	plan.Hash = planHash(plan)
	return plan, nil
}

func pruneUnchangedFiles(files map[string]fileChange) {
	for path, ch := range files {
		if fileChangeChanged(ch) {
			continue
		}
		delete(files, path)
	}
}

func fileChangesChanged(files map[string]fileChange) bool {
	for _, ch := range files {
		if fileChangeChanged(ch) {
			return true
		}
	}
	return false
}

func fileChangeChanged(ch fileChange) bool {
	return !bytesStateEqual(ch.Before, ch.AbsentBefore, ch.After, ch.AbsentAfter)
}

func planOne(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	switch op.Kind {
	case "guard":
		return planGuard(w, files, op)
	case "file":
		return planFileOp(w, files, op)
	case "structured":
		return planStructured(w, files, op)
	case "jsonl":
		return planJSONLAppend(w, files, op)
	case "md-section":
		return planSection(w, files, op)
	case "md-field":
		return planMarkdownField(w, files, op)
	case "md-task-list":
		return planMarkdownTaskList(w, files, op)
	case "table":
		return planTable(w, files, op)
	default:
		return op, false, usagef("unsupported operation kind %s", op.Kind)
	}
}

func ensureFileState(w *Workspace, files map[string]fileChange, path string, requireExists bool, finalSymlink bool) (fileChange, ResolvedPath, error) {
	res, err := w.Resolve(path, false, finalSymlink)
	if err != nil {
		return fileChange{}, res, err
	}
	if ch, ok := files[res.Clean]; ok {
		if requireExists && ch.AbsentAfter {
			return ch, res, failf("%s is missing", path)
		}
		return ch, res, nil
	}
	before, mode, absent, err := w.ReadBase(res)
	if err != nil {
		return fileChange{}, res, err
	}
	if requireExists && absent {
		return fileChange{}, res, failf("%s is missing", path)
	}
	ch := fileChange{Path: res.Clean, RepoPath: res.Repo, AbsPath: res.Abs, Before: before, After: append([]byte(nil), before...), Mode: mode, AbsentBefore: absent, AbsentAfter: absent}
	files[res.Clean] = ch
	return ch, res, nil
}

func ensureCreateDestinationState(w *Workspace, files map[string]fileChange, path string) (fileChange, ResolvedPath, error) {
	res, err := w.Resolve(path, true, false)
	if err != nil {
		return fileChange{}, res, err
	}
	if ch, ok := files[res.Clean]; ok {
		return ch, res, nil
	}
	before, mode, absent, err := w.ReadBase(res)
	if err != nil {
		return fileChange{}, res, err
	}
	ch := fileChange{Path: res.Clean, RepoPath: res.Repo, AbsPath: res.Abs, Before: before, After: append([]byte(nil), before...), Mode: mode, AbsentBefore: absent, AbsentAfter: absent}
	files[res.Clean] = ch
	return ch, res, nil
}

func setFileState(files map[string]fileChange, ch fileChange, after []byte, absent bool) (bool, fileChange) {
	ch.After = after
	ch.AbsentAfter = absent
	files[ch.Path] = ch
	return !bytes.Equal(ch.Before, ch.After) || ch.AbsentBefore != ch.AbsentAfter, ch
}

func planGuard(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	res, err := w.Resolve(op.Path, false, true)
	if err != nil {
		return op, false, err
	}
	exists, b, _, err := w.ExistsInAdmittedView(res)
	if err != nil {
		return op, false, err
	}
	op.Path = res.Clean
	op.Target = PlanTarget{Path: res.Clean}
	switch op.Verb {
	case "exists":
		if !exists {
			return op, false, failf("guard failed: exists %s", res.Clean)
		}
	case "missing":
		if exists {
			return op, false, failf("guard failed: missing %s", res.Clean)
		}
	case "contains":
		if !exists || !bytes.Contains(b, []byte(op.Value)) {
			return op, false, failf("guard failed: contains %s", res.Clean)
		}
	}
	fillDescriptor(&op)
	return op, false, nil
}

func planFileOp(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	switch op.Verb {
	case "create":
		ch, res, err := ensureCreateDestinationState(w, files, op.Path)
		if err != nil {
			return op, false, err
		}
		op.Path = res.Clean
		op.Target = PlanTarget{Path: res.Clean}
		desired := []byte(op.Value)
		if !ch.AbsentAfter {
			if bytes.Equal(ch.After, desired) {
				op.Noop = true
				fillDescriptor(&op)
				return op, false, nil
			}
			return op, false, failf("%s already exists with different content", res.Clean)
		}
		ch.Mode = "100644"
		changed, _ := setFileState(files, ch, desired, false)
		fillDescriptor(&op)
		return op, changed, nil
	case "replace":
		ch, res, err := ensureFileState(w, files, op.Path, true, false)
		if err != nil {
			return op, false, err
		}
		if !strings.HasPrefix(ch.Mode, "100") {
			return op, false, failf("%s is not a regular file", res.Clean)
		}
		op.Path = res.Clean
		op.Target = PlanTarget{Path: res.Clean}
		changed, _ := setFileState(files, ch, []byte(op.Value), false)
		op.Noop = !changed
		fillDescriptor(&op)
		return op, changed, nil
	case "delete":
		ch, res, err := ensureFileState(w, files, op.Path, false, false)
		if err != nil {
			return op, false, err
		}
		op.Path = res.Clean
		op.Target = PlanTarget{Path: res.Clean}
		changed, _ := setFileState(files, ch, nil, true)
		op.Noop = !changed
		fillDescriptor(&op)
		return op, changed, nil
	case "copy", "move":
		src, srcRes, err := ensureFileState(w, files, op.Path, true, false)
		if err != nil {
			return op, false, err
		}
		dstRes, err := w.Resolve(op.Value, true, false)
		if err != nil {
			return op, false, err
		}
		if existing, _, _, err := w.ExistsInAdmittedView(dstRes); err != nil {
			return op, false, err
		} else if existing {
			return op, false, failf("%s already exists", dstRes.Clean)
		}
		if ch, ok := files[dstRes.Clean]; ok && !ch.AbsentAfter {
			return op, false, failf("%s already exists in transaction", dstRes.Clean)
		}
		dst := fileChange{Path: dstRes.Clean, RepoPath: dstRes.Repo, AbsPath: dstRes.Abs, Before: nil, After: append([]byte(nil), src.After...), Mode: src.Mode, AbsentBefore: true, AbsentAfter: false}
		files[dstRes.Clean] = dst
		changed := true
		if op.Verb == "move" {
			c, updated := setFileState(files, src, nil, true)
			_ = updated
			changed = changed || c
		}
		op.Path = srcRes.Clean
		op.Value = dstRes.Clean
		op.Target = PlanTarget{Path: srcRes.Clean}
		fillDescriptor(&op)
		return op, changed, nil
	default:
		return op, false, usagef("unknown file verb %s", op.Verb)
	}
}

func planStructured(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, false, true)
	if err != nil {
		if op.Target.Part == "frontmatter" && (op.Verb == "delete" || op.Verb == "remove") {
			op.Noop = true
			return op, false, nil
		}
		return op, false, err
	}
	if ch.AbsentAfter {
		switch op.Verb {
		case "delete", "remove":
			op.Path = res.Clean
			op.Target.Path = res.Clean
			op.RepoPath = res.Repo
			op.Noop = true
			fillDescriptor(&op)
			return op, false, nil
		case "set", "append", "add":
			if isJSONPath(res.Clean) {
				ch.After = []byte("{}\n")
				ch.AbsentAfter = false
			} else if isYAMLPath(res.Clean) {
				ch.After = []byte{}
				ch.AbsentAfter = false
			} else if op.Target.Part == "frontmatter" && isMarkdownPath(res.Clean) {
				ch.After = []byte{}
				ch.AbsentAfter = false
			} else {
				return op, false, failf("%s is missing", res.Clean)
			}
		}
	}
	out, changed, err := evalStructuredBytes(res.Clean, op.Target.Part, op.Target.Selector, op.Verb, op.Value, op.ValueMode, ch.After)
	if err != nil {
		return op, false, err
	}
	c, updated := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = !changed && !c
	op.ValueHash = valueHash(op)
	fillDescriptor(&op)
	return op, changed || c || !bytes.Equal(updated.Before, updated.After), nil
}

func valueHash(op Operation) string {
	if op.ValueMode == "" {
		return shaHex([]byte(op.Value))
	}
	return shaHex([]byte(string(op.ValueMode) + "\x00" + op.Value))
}

func planJSONLAppend(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, false, true)
	if err != nil {
		return op, false, err
	}
	if ch.AbsentAfter {
		ch.After = nil
		ch.AbsentAfter = false
	}
	out, changed, err := evalJSONLAppend(op.Value, ch.After)
	if err != nil {
		return op, false, err
	}
	c, _ := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = false
	fillDescriptor(&op)
	return op, changed || c, nil
}

func planSection(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, true, true)
	if err != nil {
		return op, false, err
	}
	out, changed, err := evalMarkdownSection(res.Clean, op.Verb, op.Target.Section, op.Value, ch.After)
	if err != nil {
		return op, false, err
	}
	c, _ := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = !changed && !c
	fillDescriptor(&op)
	return op, changed || c, nil
}

func planMarkdownField(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, true, true)
	if err != nil {
		return op, false, err
	}
	out, changed, err := evalMarkdownField(res.Clean, op, ch.After)
	if err != nil {
		return op, false, err
	}
	c, _ := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = !changed && !c
	fillDescriptor(&op)
	return op, changed || c, nil
}

func planMarkdownTaskList(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, true, true)
	if err != nil {
		return op, false, err
	}
	out, changed, err := evalMarkdownTaskList(res.Clean, op, ch.After)
	if err != nil {
		return op, false, err
	}
	c, _ := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = !changed && !c
	fillDescriptor(&op)
	return op, changed || c, nil
}

func planTable(w *Workspace, files map[string]fileChange, op Operation) (Operation, bool, error) {
	ch, res, err := ensureFileState(w, files, op.Path, true, true)
	if err != nil {
		return op, false, err
	}
	out, changed, err := evalTable(res.Clean, op, ch.After)
	if err != nil {
		return op, false, err
	}
	c, _ := setFileState(files, ch, out, false)
	op.Path = res.Clean
	op.Target.Path = res.Clean
	op.RepoPath = res.Repo
	op.Noop = !changed && !c
	fillDescriptor(&op)
	return op, changed || c, nil
}

func planHash(plan *Plan) string {
	return "sha256:" + shaHex(canonicalPlanBytes(plan))
}

func canonicalPlanBytes(plan *Plan) []byte {
	type planAlias Plan
	cp := planAlias(*plan)
	cp.Hash = ""
	cp.Touched = nil
	cp.Mutating = false
	cp.Changed = false
	b, err := jsonx.Marshal(cp)
	if err != nil {
		panic(fmt.Sprintf("marshal internal plan: %v", err))
	}
	canonical, err := jcs.Canonicalize(b)
	if err != nil {
		panic(fmt.Sprintf("canonicalize internal plan: %v", err))
	}
	return canonical
}

func buildCommitMessage(opts GlobalOptions, ops []Operation) string {
	if opts.Message != "" {
		return opts.Message
	}
	var mut []Operation
	for _, op := range ops {
		if op.Class != ClassGuard && !op.Noop {
			mut = append(mut, op)
		}
	}
	if len(mut) == 0 {
		for _, op := range ops {
			if op.Class != ClassGuard {
				mut = append(mut, op)
			}
		}
	}
	msg := "etch: no changes"
	if len(mut) == 1 {
		subj := "etch " + mut[0].Descriptor
		if commitSubjectFits(subj, opts) {
			msg = subj
		} else {
			msg = "etch " + descriptorWithoutValue(mut[0])
			if operationHasValue(mut[0]) {
				msg += "\n\nValue: " + valuePreview(mut[0].Value, mut[0].ValueMode, 80)
			}
		}
	} else if len(mut) > 1 {
		paths := map[string]bool{}
		for _, op := range mut {
			paths[op.Target.Path] = true
		}
		if len(paths) == 1 {
			for p := range paths {
				msg = fmt.Sprintf("etch: %d changes in %s", len(mut), p)
			}
		} else {
			msg = fmt.Sprintf("etch: %d changes across %d files", len(mut), len(paths))
		}
		msg += "\n\nChanges:"
		for _, op := range mut {
			msg += "\n- " + op.Descriptor
		}
	}
	return applyCommitMessageModifiers(msg, opts)
}

func commitSubjectFits(subject string, opts GlobalOptions) bool {
	subject = opts.SubjectPrefix + subject + opts.SubjectSuffix
	return len(subject) <= 72 && !strings.Contains(subject, "\n")
}

func applyCommitMessageModifiers(msg string, opts GlobalOptions) string {
	subject, body := splitCommitMessage(msg)
	subject = opts.SubjectPrefix + subject + opts.SubjectSuffix
	body = joinCommitBodyBlocks(opts.BodyPrefix, body, opts.BodySuffix)
	if body == "" {
		return subject
	}
	return subject + "\n\n" + body
}

func splitCommitMessage(msg string) (subject, body string) {
	subject, body, ok := strings.Cut(msg, "\n\n")
	if !ok {
		return msg, ""
	}
	return subject, body
}

func joinCommitBodyBlocks(blocks ...string) string {
	var out string
	for _, block := range blocks {
		if block == "" {
			continue
		}
		if out == "" {
			out = block
			continue
		}
		out = strings.TrimRight(out, "\n") + "\n\n" + strings.TrimLeft(block, "\n")
	}
	return out
}

func descriptorWithoutValue(op Operation) string {
	if !operationHasValue(op) {
		return op.Descriptor
	}
	pv := valuePreview(op.Value, op.ValueMode, 80)
	return strings.TrimSpace(strings.TrimSuffix(op.Descriptor, pv))
}

func operationHasValue(op Operation) bool {
	return (op.Kind == "structured" && op.Verb != "delete") || op.Value != ""
}
