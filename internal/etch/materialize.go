package etch

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Materializer struct {
	w      *Workspace
	plan   *Plan
	skip   bool
	states map[string]preMaterialize
}

type preMaterialize struct {
	index          []byte
	indexAbsent    bool
	worktree       []byte
	worktreeAbsent bool
	// gitClean uses Git's conversion-aware status, so CRLF/smudge-clean paths
	// can still take the Git checkout path when the user has no local edits.
	gitClean bool
}

func NewMaterializer(w *Workspace, plan *Plan, skip bool) (Materializer, error) {
	m := Materializer{w: w, plan: plan, skip: skip, states: map[string]preMaterialize{}}
	if skip {
		return m, nil
	}
	for _, k := range sortedKeys(plan.Touched) {
		ch := plan.Touched[k]
		idx, idxAbsent, _ := w.indexBytes(ch.RepoPath)
		wt, wtAbsent, err := w.readWorktreeFile(ch)
		if err != nil {
			return Materializer{}, err
		}
		gitClean, err := w.pathClean(ch.RepoPath)
		if err != nil {
			return Materializer{}, err
		}
		if ch.AbsentBefore && !wtAbsent {
			gitClean = false
		}
		m.states[k] = preMaterialize{index: idx, indexAbsent: idxAbsent, worktree: wt, worktreeAbsent: wtAbsent, gitClean: gitClean}
	}
	return m, nil
}

func (m Materializer) Preflight() error {
	if m.skip {
		return nil
	}
	var blocked []string
	for _, k := range sortedKeys(m.plan.Touched) {
		ch := m.plan.Touched[k]
		if m.states[k].gitClean {
			continue
		}
		reason, err := m.w.checkoutConversionRisk(ch.RepoPath)
		if err != nil {
			return err
		}
		if reason != "" {
			blocked = append(blocked, fmt.Sprintf("%s (%s)", ch.Path, reason))
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	return failf("dirty checkout conversion path cannot be safely materialized: %s; clean the path or rerun with --no-checkout", strings.Join(blocked, "; "))
}

func (m Materializer) Apply(commit string, stderr interface {
	Write([]byte) (int, error)
}) error {
	if m.skip {
		return nil
	}
	var conflicts []string
	var failures []string
	for _, k := range sortedKeys(m.plan.Touched) {
		ch := m.plan.Touched[k]
		state := m.states[k]
		result, err := m.materializeOne(commit, ch, state)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", ch.Path, err))
			continue
		}
		if result.conflict {
			conflicts = append(conflicts, ch.Path)
		}
	}
	if len(conflicts) > 0 {
		fmt.Fprintf(stderr, "etch: committed %.7s, but materialization wrote conflicts in %d paths\n", commit, len(conflicts))
		fmt.Fprintln(stderr, "etch: HEAD was updated; the commit exists and will not be rolled back")
		fmt.Fprintln(stderr, "etch: conflicted paths:")
		for _, p := range conflicts {
			fmt.Fprintf(stderr, "etch:   %s\n", p)
		}
		fmt.Fprintln(stderr, "etch: resolve conflict markers, then commit or discard the working-tree resolution")
		fmt.Fprintln(stderr, "etch: run `etch help conflicts` for recovery steps")
		return failf("materialization wrote conflicts")
	}
	if len(failures) > 0 {
		fmt.Fprintf(stderr, "etch: committed %.7s, but could not materialize %d paths\n", commit, len(failures))
		fmt.Fprintln(stderr, "etch: HEAD was updated; the commit exists and will not be rolled back")
		fmt.Fprintln(stderr, "etch: failed paths:")
		for _, p := range failures {
			fmt.Fprintf(stderr, "etch:   %s\n", p)
		}
		fmt.Fprintln(stderr, "etch: working tree and index may not match HEAD for these paths")
		fmt.Fprintln(stderr, "etch: run `etch help conflicts` for recovery steps")
		return failf("materialization failed")
	}
	return nil
}

type materializeResult struct {
	conflict bool
}

func (m Materializer) materializeOne(commit string, ch fileChange, st preMaterialize) (materializeResult, error) {
	if st.gitClean {
		// Clean paths can let Git own checkout conversion and index stat updates.
		return materializeResult{}, m.w.restorePathFromCommit(commit, ch.RepoPath)
	}

	base := ch.Before
	baseAbsent := ch.AbsentBefore
	newBytes := ch.After
	newAbsent := ch.AbsentAfter

	indexClean := bytesStateEqual(st.index, st.indexAbsent, base, baseAbsent)
	worktreeClean := bytesStateEqual(st.worktree, st.worktreeAbsent, base, baseAbsent)
	if indexClean && worktreeClean {
		if err := m.writeBoth(ch, newBytes, newAbsent); err != nil {
			return materializeResult{}, err
		}
		return materializeResult{}, nil
	}

	postIndex, postIndexAbsent, indexConflict, err := mergeState(base, baseAbsent, newBytes, newAbsent, st.index, st.indexAbsent, "HEAD", "index")
	if err != nil {
		return materializeResult{}, err
	}
	if indexConflict {
		if err := m.writeIndexHeadAndWorktree(ch, postIndex, postIndexAbsent); err != nil {
			return materializeResult{}, err
		}
		return materializeResult{conflict: true}, nil
	}
	postWT, postWTAbsent, wtConflict, err := mergeState(st.index, st.indexAbsent, postIndex, postIndexAbsent, st.worktree, st.worktreeAbsent, "index", "worktree")
	if err != nil {
		return materializeResult{}, err
	}
	if err := m.w.updateIndexBytes(ch.RepoPath, ch.Mode, postIndex, postIndexAbsent); err != nil {
		return materializeResult{}, err
	}
	if err := m.w.writeWorktree(ch, postWT, postWTAbsent); err != nil {
		return materializeResult{}, err
	}
	return materializeResult{conflict: wtConflict}, nil
}

func (m Materializer) writeBoth(ch fileChange, b []byte, absent bool) error {
	if err := m.w.updateIndexBytes(ch.RepoPath, ch.Mode, b, absent); err != nil {
		return err
	}
	return m.w.writeWorktree(ch, b, absent)
}

func (m Materializer) writeIndexHeadAndWorktree(ch fileChange, conflict []byte, absent bool) error {
	if err := m.w.updateIndexBytes(ch.RepoPath, ch.Mode, ch.After, ch.AbsentAfter); err != nil {
		return err
	}
	return m.w.writeWorktree(ch, conflict, absent)
}

func bytesStateEqual(a []byte, aAbsent bool, b []byte, bAbsent bool) bool {
	if aAbsent || bAbsent {
		return aAbsent == bAbsent
	}
	return bytes.Equal(a, b)
}

func mergeState(base []byte, baseAbsent bool, ours []byte, oursAbsent bool, theirs []byte, theirsAbsent bool, oursLabel, theirsLabel string) ([]byte, bool, bool, error) {
	if bytesStateEqual(theirs, theirsAbsent, base, baseAbsent) {
		return ours, oursAbsent, false, nil
	}
	if bytesStateEqual(theirs, theirsAbsent, ours, oursAbsent) {
		return ours, oursAbsent, false, nil
	}
	if bytesStateEqual(ours, oursAbsent, base, baseAbsent) {
		return theirs, theirsAbsent, false, nil
	}
	if baseAbsent || oursAbsent || theirsAbsent {
		switch {
		case baseAbsent && !oursAbsent && !theirsAbsent:
			if !textMergeable(ours, false) || !textMergeable(theirs, false) {
				break
			}
			return conflictBytes(nil, ours, theirs, oursLabel, theirsLabel), false, true, nil
		case oursAbsent && !baseAbsent && !theirsAbsent:
			if !textMergeable(base, false) || !textMergeable(theirs, false) {
				break
			}
			return conflictBytes(base, nil, theirs, oursLabel, theirsLabel), false, true, nil
		case theirsAbsent && !baseAbsent && !oursAbsent:
			if !textMergeable(base, false) || !textMergeable(ours, false) {
				break
			}
			return conflictBytes(base, ours, nil, oursLabel, theirsLabel), false, true, nil
		}
		return nil, false, false, failf("binary local change could not be merged")
	}

	if !textMergeable(base, false) || !textMergeable(ours, false) || !textMergeable(theirs, false) {
		return nil, false, false, failf("binary local change could not be merged")
	}
	merged, clean := simpleThreeWay(base, ours, theirs)
	if clean {
		return merged, false, false, nil
	}
	return conflictBytes(base, ours, theirs, oursLabel, theirsLabel), false, true, nil
}

func textMergeable(b []byte, absent bool) bool {
	if absent {
		return true
	}
	return utf8.Valid(b) && !bytes.Contains(b, []byte{0})
}

func simpleThreeWay(base, ours, theirs []byte) ([]byte, bool) {
	if bytes.Equal(ours, theirs) {
		return ours, true
	}
	baseLines := splitLines(string(base))
	oursLines := splitLines(string(ours))
	theirsLines := splitLines(string(theirs))
	merged, ok := mergeLineEdits(baseLines, lineEdits(baseLines, oursLines), lineEdits(baseLines, theirsLines))
	if !ok {
		return nil, false
	}
	return []byte(strings.Join(merged, "")), true
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.SplitAfter(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

type lineEdit struct {
	start int
	end   int
	repl  []string
}

func lineEdits(base, changed []string) []lineEdit {
	lcs := lcsLengths(base, changed)
	var edits []lineEdit
	i, j := 0, 0
	for i < len(base) || j < len(changed) {
		if i < len(base) && j < len(changed) && base[i] == changed[j] {
			i++
			j++
			continue
		}
		startI, startJ := i, j
		for i < len(base) || j < len(changed) {
			if i < len(base) && j < len(changed) && base[i] == changed[j] {
				break
			}
			if j < len(changed) && (i == len(base) || lcs[i][j+1] >= lcs[i+1][j]) {
				j++
			} else {
				i++
			}
		}
		edits = append(edits, lineEdit{start: startI, end: i, repl: append([]string(nil), changed[startJ:j]...)})
	}
	return edits
}

func lcsLengths(a, b []string) [][]int {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	return lcs
}

func mergeLineEdits(base []string, ours, theirs []lineEdit) ([]string, bool) {
	var merged []string
	pos := 0
	i, j := 0, 0
	appendBase := func(to int) {
		merged = append(merged, base[pos:to]...)
		pos = to
	}
	apply := func(edit lineEdit) bool {
		if edit.start < pos {
			return false
		}
		appendBase(edit.start)
		merged = append(merged, edit.repl...)
		pos = edit.end
		return true
	}

	for i < len(ours) || j < len(theirs) {
		if j >= len(theirs) || (i < len(ours) && ours[i].start < theirs[j].start) {
			if j < len(theirs) && ours[i].end > theirs[j].start {
				return nil, false
			}
			if !apply(ours[i]) {
				return nil, false
			}
			i++
			continue
		}
		if i >= len(ours) || theirs[j].start < ours[i].start {
			if i < len(ours) && theirs[j].end > ours[i].start {
				return nil, false
			}
			if !apply(theirs[j]) {
				return nil, false
			}
			j++
			continue
		}

		if ours[i].end != theirs[j].end || !stringSlicesEqual(ours[i].repl, theirs[j].repl) {
			return nil, false
		}
		if !apply(ours[i]) {
			return nil, false
		}
		i++
		j++
	}
	appendBase(len(base))
	return merged, true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func conflictBytes(base, ours, theirs []byte, oursLabel, theirsLabel string) []byte {
	var b strings.Builder
	b.WriteString("<<<<<<< ")
	b.WriteString(oursLabel)
	b.WriteByte('\n')
	b.Write(ours)
	if len(ours) > 0 && ours[len(ours)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString("||||||| base\n")
	b.Write(base)
	if len(base) > 0 && base[len(base)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString("=======\n")
	b.Write(theirs)
	if len(theirs) > 0 && theirs[len(theirs)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString(">>>>>>> ")
	b.WriteString(theirsLabel)
	b.WriteByte('\n')
	return []byte(b.String())
}
