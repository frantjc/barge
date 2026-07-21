//go:build acr

package barge_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	"github.com/stretchr/testify/require"
)

func TestCopyACR(t *testing.T) {
	ctx := Context(t)

	acrName := os.Getenv("ACR_NAME")
	require.NotEmpty(t, acrName)

	archiveChart, archiveURL := Archive(t)

	acr := fmt.Sprintf("oci://%s.azurecr.io/charts/%s", acrName, archiveChart.Name())
	acrWithTag := fmt.Sprintf("%s:%s", acr, archiveChart.Metadata.Version)

	require.NoError(t, barge.Copy(ctx, archiveURL.String(), acr))
	require.NoError(t, barge.Copy(ctx, acr, t.TempDir()))
	require.NoError(t, barge.Copy(ctx, archiveURL.String(), acrWithTag))
	require.NoError(t, barge.Copy(ctx, acrWithTag, t.TempDir()))
}
