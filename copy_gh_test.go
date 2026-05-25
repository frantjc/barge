//go:build github

package barge_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	"github.com/stretchr/testify/require"
)

func TestCopyGHCR(t *testing.T) {
	ctx := Context(t)

	githubRepository := os.Getenv("GITHUB_REPOSITORY")
	require.NotEmpty(t, githubRepository)

	archiveChart, archive := Archive(t)

	ghcr := fmt.Sprintf("oci://ghcr.io/%s/charts/%s", githubRepository, archiveChart.Name())
	ghcrWithTag := fmt.Sprintf("%s:%s", ghcr, archiveChart.Metadata.Version)

	require.NoError(t, barge.Copy(ctx, archive, ghcr))
	require.NoError(t, barge.Copy(ctx, ghcr, t.TempDir()))
	require.NoError(t, barge.Copy(ctx, archive, ghcrWithTag))
	require.NoError(t, barge.Copy(ctx, ghcrWithTag, t.TempDir()))
}
