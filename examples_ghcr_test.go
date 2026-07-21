//go:build examples && ghcr

package barge_test

import (
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	"github.com/stretchr/testify/require"
)

func TestExampleOCI(t *testing.T) {
	ctx := Context(t)
	require.NoError(t, barge.Copy(ctx, "oci://ghcr.io/frantjc/barge/charts/test", t.TempDir()))
}
