//go:build dagger

package barge_test

import (
	"fmt"
	"net/url"
	"os/exec"
	"testing"

	"dagger.io/dagger"
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

	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})

	archiveChart, archive := Archive(t)

	chartmuseumURL := Chartmuseum(t, dag)
	require.NoError(t, barge.Copy(ctx, archive, chartmuseumURL.String()))

	helm, err := exec.LookPath("helm")
	require.NoError(t, err)
	repo := "chartmuseum"
	repoURL := url.URL{
		Scheme: "http",
		Host:   chartmuseumURL.Host,
	}
	add := Command(t, helm, "repo", "add", repo, repoURL.String())
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

func TestCopyOCI(t *testing.T) {
	ctx := Context(t)

	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})

	_, archive := Archive(t)

	registryURL := Registry(t, dag)
	oci := registryURL.JoinPath("test")
	require.NoError(t, barge.Copy(ctx, archive, oci.String()))
	require.NoError(t, barge.Copy(ctx, oci.String(), t.TempDir()))

	ociWithTag := registryURL.JoinPath("test:tag")
	require.NoError(t, barge.Copy(ctx, archive, ociWithTag.String()))
	require.NoError(t, barge.Copy(ctx, ociWithTag.String(), t.TempDir()))
}
