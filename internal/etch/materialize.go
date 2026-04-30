package etch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
}

func NewMaterializer(w *Workspace, plan *Plan, skip bool) Materializer {
	m := Materializer{w: w, plan: plan, skip: skip, states: map[string]preMaterialize{}}
	for _, k := range sortedKeys(plan.Touched) {
		ch := plan.Touched[k]
		idx, idxAbsent, _ := w.indexBytes(ch.RepoPath)
		wt, wtAbsent := readWorkingTree(ch)
		m.states[k] = preMaterialize{index: idx, indexAbsent: idxAbsent, worktree: wt, worktreeAbsent: wtAbsent}
	}
	return m
}

func readWorkingTree(ch fileChange) ([]byte, bool) {
	b, err := os.ReadFile(ch.Path)
	if err == nil {
		return b, false
	}
	return nil, true
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
		result, err := m.materializeOne(ch, state)
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

func (m Materializer) materializeOne(ch fileChange, st preMaterialize) (materializeResult, error) {
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
	if err := writeWorktree(ch.Path, postWT, postWTAbsent); err != nil {
		return materializeResult{}, err
	}
	return materializeResult{conflict: wtConflict}, nil
}

func (m Materializer) writeBoth(ch fileChange, b []byte, absent bool) error {
	if err := m.w.updateIndexBytes(ch.RepoPath, ch.Mode, b, absent); err != nil {
		return err
	}
	return writeWorktree(ch.Path, b, absent)
}

func (m Materializer) writeIndexHeadAndWorktree(ch fileChange, conflict []byte, absent bool) error {
	if err := m.w.updateIndexBytes(ch.RepoPath, ch.Mode, ch.After, ch.AbsentAfter); err != nil {
		return err
	}
	return writeWorktree(ch.Path, conflict, absent)
}

func writeWorktree(path string, b []byte, absent bool) error {
	if absent {
		if err := os.Remove(path); err != nil && !isNoSuch(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
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
	prefix := commonPrefix3(baseLines, oursLines, theirsLines)
	suffix := commonSuffix3(baseLines[prefix:], oursLines[prefix:], theirsLines[prefix:])
	baseMid := strings.Join(baseLines[prefix:len(baseLines)-suffix], "")
	oursMid := strings.Join(oursLines[prefix:len(oursLines)-suffix], "")
	theirsMid := strings.Join(theirsLines[prefix:len(theirsLines)-suffix], "")
	if baseMid == theirsMid {
		return []byte(strings.Join(append(append([]string{}, oursLines[:len(oursLines)-suffix]...), oursLines[len(oursLines)-suffix:]...), "")), true
	}
	if baseMid == oursMid {
		return theirs, true
	}
	return nil, false
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

func commonPrefix3(a, b, c []string) int {
	n := 0
	for n < len(a) && n < len(b) && n < len(c) && a[n] == b[n] && a[n] == c[n] {
		n++
	}
	return n
}

func commonSuffix3(a, b, c []string) int {
	n := 0
	for n < len(a) && n < len(b) && n < len(c) && a[len(a)-1-n] == b[len(b)-1-n] && a[len(a)-1-n] == c[len(c)-1-n] {
		n++
	}
	return n
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
