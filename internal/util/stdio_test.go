package util_test

import (
	"io"
	"strings"
	"testing"

	"github.com/frantjc/barge/internal/util"
)

func TestStdinFrom(t *testing.T) {
	t.Parallel()

	// Without stdin in the context, nothing is read, rather than the
	// process's own stdin.
	b, err := io.ReadAll(util.StdinFrom(t.Context()))
	if err != nil || len(b) != 0 {
		t.Fatalf("expected an empty reader, got %q, %v", b, err)
	}

	ctx := util.StdinInto(t.Context(), strings.NewReader("hello"))
	b, err = io.ReadAll(util.StdinFrom(ctx))
	if err != nil || string(b) != "hello" {
		t.Fatalf("expected %q, got %q, %v", "hello", b, err)
	}
}
