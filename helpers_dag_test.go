//go:build dagger

package barge_test

import (
	"context"
	"net/url"
	"testing"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
)

func Chartmuseum(t testing.TB, dag *dagger.Client) *url.URL {
	t.Helper()
	ctx := t.Context()
	chartmuseum, err := dag.Container().
		From("ghcr.io/helm/chartmuseum:v0.16.3").
		WithExposedPort(8080).
		WithEnvVariable("DEBUG", "1").
		WithEnvVariable("STORAGE", "local").
		WithEnvVariable("STORAGE_LOCAL_ROOTDIR", "/tmp").
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = chartmuseum.Stop(context.WithoutCancel(ctx))
		require.NoError(t, err)
	})
	rawChartmuseumURL, err := chartmuseum.Endpoint(ctx, dagger.ServiceEndpointOpts{Scheme: "chartmuseum+http"})
	require.NoError(t, err)
	chartmuseumURL, err := url.Parse(rawChartmuseumURL)
	require.NoError(t, err)
	return chartmuseumURL
}

func Registry(t testing.TB, dag *dagger.Client) *url.URL {
	t.Helper()
	ctx := t.Context()
	registry, err := dag.Container().
		From("docker.io/distribution/distribution:3").
		WithExposedPort(5000).
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = registry.Stop(context.WithoutCancel(ctx))
		require.NoError(t, err)
	})
	rawRegistryURL, err := registry.Endpoint(ctx, dagger.ServiceEndpointOpts{Scheme: "oci"})
	require.NoError(t, err)
	registryURL, err := url.Parse(rawRegistryURL)
	require.NoError(t, err)
	return registryURL
}
