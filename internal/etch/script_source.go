package etch

import (
	"io"
	"os"
	"path/filepath"
)

// scriptSource is the IO boundary for etch scripts. The parser works on bytes;
// this source keeps command-line script loading injectable and separate from
// CLI argument handling.
type scriptSource interface {
	readFile(path string) ([]byte, error)
	readStdin() ([]byte, error)
}

type osScriptSource struct{}

func ParseScriptAt(cwd, path string) ([]Statement, error) {
	return parseScriptAt(cwd, path, osScriptSource{})
}

func parseScriptAt(cwd, path string, source scriptSource) ([]Statement, error) {
	if source == nil {
		source = osScriptSource{}
	}
	var data []byte
	var err error
	name := path
	if path == "-" {
		name = "<stdin>"
		data, err = source.readStdin()
	} else {
		readPath := path
		if cwd != "" && !filepath.IsAbs(readPath) {
			readPath = filepath.Join(cwd, readPath)
		}
		data, err = source.readFile(readPath)
	}
	if err != nil {
		return nil, failf("%s", err)
	}
	return ParseScriptBytes(name, data)
}

func (osScriptSource) readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osScriptSource) readStdin() ([]byte, error) {
	return readStdin()
}

var readStdin = func() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
