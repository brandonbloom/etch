package etch

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyErrUsesErrorsAs(t *testing.T) {
	wrapped := fmt.Errorf("wrapped: %w", usagef("bad usage"))
	if got := classifyErr(wrapped); got != exitUsage {
		t.Fatalf("classifyErr(wrapped usage) = %d, want %d", got, exitUsage)
	}

	joined := errors.Join(fmt.Errorf("other"), usagef("bad usage"))
	if got := classifyErr(joined); got != exitUsage {
		t.Fatalf("classifyErr(joined usage) = %d, want %d", got, exitUsage)
	}
}
