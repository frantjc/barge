package util_test

import (
	"io"
	"strings"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/require"
)

func TestStdinFrom(t *testing.T) {
	t.Parallel()

	// Without stdin in the context, nothing is read, rather than the
	// process's own stdin.
	b, err := io.ReadAll(util.StdinFrom(t.Context()))
	require.NoError(t, err)
	require.Empty(t, b)

	ctx := util.StdinInto(t.Context(), strings.NewReader("hello"))
	b, err = io.ReadAll(util.StdinFrom(ctx))
	require.NoError(t, err)
	require.Equal(t, "hello", string(b))
}
