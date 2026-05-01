package etch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brandonbloom/etch/internal/jsonx"
)

const (
	versionString = "etch 0.1.0"
	planSchema    = "https://brandonbloom.github.io/etch/schemas/2026-04/plan.schema.json"
)

type exitCode int

const (
	exitOK      exitCode = 0
	exitFailure exitCode = 1
	exitUsage   exitCode = 2
)

type errWithCode struct {
	code exitCode
	err  error
}

func (e errWithCode) Error() string { return e.err.Error() }

func usagef(format string, args ...any) error {
	return errWithCode{code: exitUsage, err: fmt.Errorf(format, args...)}
}

func failf(format string, args ...any) error {
	return errWithCode{code: exitFailure, err: fmt.Errorf(format, args...)}
}

type GlobalOptions struct {
	CWD           string
	Plan          bool
	DryRun        bool
	NoCheckout    bool
	Untracked     bool
	Message       string
	MessagePrefix string
	MessageSuffix string
	Retries       int
	AllowEmpty    bool
}

type SourceLoc struct {
	Name string
	Line int
}

func (s SourceLoc) Prefix() string {
	if s.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d: ", s.Name, s.Line)
}

type Statement struct {
	Tokens []string
	Loc    SourceLoc
}

type CommandClass string

const (
	ClassGuard         CommandClass = "guard"
	ClassIdempotent    CommandClass = "idempotent"
	ClassNonIdempotent CommandClass = "non-idempotent"
	ClassIntrospection CommandClass = "introspection"
)

type Operation struct {
	Verb       string       `json:"verb"`
	Kind       string       `json:"-"`
	Class      CommandClass `json:"-"`
	Raw        []string     `json:"-"`
	Loc        SourceLoc    `json:"-"`
	Path       string       `json:"-"`
	RepoPath   string       `json:"-"`
	Target     PlanTarget   `json:"target,omitempty"`
	Value      string       `json:"-"`
	ValueHash  string       `json:"value_sha256,omitempty"`
	Noop       bool         `json:"-"`
	Descriptor string       `json:"-"`
}

type PlanTarget struct {
	Path     string `json:"path,omitempty"`
	Part     string `json:"part,omitempty"`
	Selector string `json:"selector,omitempty"`
	Section  string `json:"section,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Table    string `json:"table,omitempty"`
	Range    string `json:"range,omitempty"`
	Column   string `json:"column,omitempty"`
	Row      string `json:"row,omitempty"`
}

type PlanFile struct {
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

type PlanCommit struct {
	Message string `json:"message"`
}

type Plan struct {
	Schema     string                `json:"$schema"`
	Ref        string                `json:"ref"`
	BaseCommit string                `json:"base_commit"`
	Operations []Operation           `json:"operations"`
	Files      map[string]PlanFile   `json:"files"`
	Tree       string                `json:"tree"`
	Commit     PlanCommit            `json:"commit"`
	Hash       string                `json:"-"`
	Touched    map[string]fileChange `json:"-"`
	Mutating   bool                  `json:"-"`
	Changed    bool                  `json:"-"`
}

type fileChange struct {
	Path         string
	RepoPath     string
	Before       []byte
	After        []byte
	Mode         string
	AbsentBefore bool
	AbsentAfter  bool
}

func shaHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func jsonOut(w io.Writer, v any) error {
	return jsonx.WriteIndented(w, v)
}

func compactJSON(v any) string {
	b, err := jsonx.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isNoSuch(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func cleanSlash(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func hasUTF8BOM(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf
}

func trimUTF8BOM(b []byte) ([]byte, bool) {
	if hasUTF8BOM(b) {
		return b[3:], true
	}
	return b, false
}

func withUTF8BOM(b []byte, had bool) []byte {
	if !had {
		return b
	}
	out := make([]byte, 0, len(b)+3)
	out = append(out, 0xef, 0xbb, 0xbf)
	out = append(out, b...)
	return out
}

func ensureTrailingNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	return append(b, '\n')
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	ok := true
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' || r == ',' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
