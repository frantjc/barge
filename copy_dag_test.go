//go:build dagger

package barge_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestCopyChartmuseum(t *testing.T) {
	ctx := Context(t)

	dag := Dag(t)
	archiveChart, archiveURL := Archive(t)
	chartmuseumURL := Chartmuseum(t, dag)
	require.NoError(t, barge.Copy(ctx, archiveURL.String(), chartmuseumURL.String()))

	repo := "chartmuseum"
	repoURL := url.URL{
		Scheme: "http",
		Host:   chartmuseumURL.Host,
	}
	add := Command(t, "helm", "repo", "add", repo, repoURL.String())
	require.NoError(t, add.Run())

	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("repo://%s/%s", repo, archiveChart.Name()), t.TempDir()))
	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("repo://%s/%s?version=%s", repo, archiveChart.Name(), archiveChart.Metadata.Version), t.TempDir()))

	httpURL := &url.URL{
		Scheme: "http",
		Host:   chartmuseumURL.Host,
		Path:   chartmuseumURL.JoinPath("charts/test-0.1.0.tgz").Path,
	}
	require.NoError(t, barge.Copy(ctx, httpURL.String(), t.TempDir()))
}

func TestCopyDistribution(t *testing.T) {
	ctx := Context(t)

	dag := Dag(t)
	oci := Distribution(t, dag).JoinPath("test")
	_, archiveURL := Archive(t)
	require.NoError(t, barge.Copy(ctx, archiveURL.String(), oci.String()))
	require.NoError(t, barge.Copy(ctx, oci.String(), t.TempDir()))

	ociWithTag := fmt.Sprintf("%s:tag", oci)
	require.NoError(t, barge.Copy(ctx, archiveURL.String(), ociWithTag))
	require.NoError(t, barge.Copy(ctx, ociWithTag, t.TempDir()))
}
