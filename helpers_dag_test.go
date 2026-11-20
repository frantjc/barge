//go:build dagger

package barge_test

import (
	"net/url"
	"testing"

	"dagger.io/dagger"
	"dagger.io/dagger/core"
	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/require"
)

func Chartmuseum(t testing.TB, dag *core.Query) *url.URL {
	t.Helper()
	ctx := t.Context()
	container := dag.Container()
	if username, password, err := util.GetGitHubAuth(ctx); err == nil {
		container = container.WithRegistryAuth("ghcr.io", username, dag.SetSecret("github-password", password))
	}
	chartmuseum, err := container.
		From("ghcr.io/helm/chartmuseum:v0.16.5").
		WithExposedPort(8080).
		WithEnvVariable("DEBUG", "1").
		WithEnvVariable("STORAGE", "local").
		WithEnvVariable("STORAGE_LOCAL_ROOTDIR", "/tmp").
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = chartmuseum.Stop(Context(t))
		require.NoError(t, err)
	})
	rawChartmuseumURL, err := chartmuseum.Endpoint(ctx, core.ServiceEndpointOpts{Scheme: "chartmuseum+http"})
	require.NoError(t, err)
	chartmuseumURL, err := url.Parse(rawChartmuseumURL)
	require.NoError(t, err)
	return chartmuseumURL
}

func Distribution(t testing.TB, dag *core.Query) *url.URL {
	t.Helper()
	ctx := t.Context()
	distribution, err := dag.Container().
		From("docker.io/distribution/distribution:3").
		WithExposedPort(5000).
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = distribution.Stop(Context(t))
		require.NoError(t, err)
	})
	rawDistributionURL, err := distribution.Endpoint(ctx, core.ServiceEndpointOpts{Scheme: "oci+http"})
	require.NoError(t, err)
	distributionURL, err := url.Parse(rawDistributionURL)
	require.NoError(t, err)
	return distributionURL
}

func Dag(t testing.TB) *core.Query {
	t.Helper()
	ctx := Context(t)
	client, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	return core.NewQuery(client)
}
