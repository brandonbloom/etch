package etch

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

func ParseScriptBytes(name string, data []byte) ([]Statement, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var stmts []Statement
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		lineNo := i + 1
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		tokens, heredocs, err := tokenizeStatement(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, lineNo, err)
		}
		for _, hd := range heredocs {
			var body []string
			found := false
			for i++; i < len(lines); i++ {
				if lines[i] == hd.delim {
					found = true
					break
				}
				body = append(body, lines[i])
			}
			if !found {
				return nil, fmt.Errorf("%s:%d: missing heredoc terminator %q", name, lineNo, hd.delim)
			}
			value := strings.Join(body, "\n")
			if len(body) > 0 {
				value += "\n"
			}
			tokens[hd.index] = value
		}
		stmts = append(stmts, Statement{Tokens: tokens, Loc: SourceLoc{Name: name, Line: lineNo}})
	}
	return stmts, nil
}

type heredocToken struct {
	index int
	delim string
}

func tokenizeStatement(line string) ([]string, []heredocToken, error) {
	var tokens []string
	var heredocs []heredocToken
	var b strings.Builder
	inSingle := false
	inDouble := false
	escaping := false
	have := false

	flush := func() {
		if have {
			tokens = append(tokens, b.String())
			b.Reset()
			have = false
		}
	}

	for _, r := range line {
		if escaping {
			b.WriteRune(r)
			have = true
			escaping = false
			continue
		}
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				b.WriteRune(r)
				have = true
			}
		case inDouble:
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaping = true
				have = true
			default:
				b.WriteRune(r)
				have = true
			}
		default:
			switch r {
			case '\'':
				inSingle = true
				have = true
			case '"':
				inDouble = true
				have = true
			case '\\':
				escaping = true
				have = true
			case ' ', '\t', '\r':
				flush()
			default:
				b.WriteRune(r)
				have = true
			}
		}
	}
	if escaping {
		return nil, nil, usagef("trailing backslash")
	}
	if inSingle {
		return nil, nil, usagef("unterminated single quote")
	}
	if inDouble {
		return nil, nil, usagef("unterminated double quote")
	}
	flush()
	for i, tok := range tokens {
		if strings.HasPrefix(tok, "<<") {
			delim := strings.TrimPrefix(tok, "<<")
			if delim == "" {
				return nil, nil, usagef("empty heredoc delimiter")
			}
			heredocs = append(heredocs, heredocToken{index: i, delim: delim})
		}
	}
	return tokens, heredocs, nil
}
