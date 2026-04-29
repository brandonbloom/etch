package main

import (
	"os"

	"github.com/brandonbloom/etch/internal/etch"
)

func main() {
	os.Exit(etch.Main(os.Args[1:], os.Stdout, os.Stderr))
}
