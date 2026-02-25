package barge_test

import (
	"context"
	"log/slog"
	"net/url"
	"os/exec"
	"testing"

	"dagger.io/dagger"
	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Command(t testing.TB, name string, arg ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, arg...)
	cmd.Stdout = t.Output()
	cmd.Stderr = t.Output()
	return cmd
}

func Context(t testing.TB) context.Context {
	t.Helper()
	return util.SloggerInto(
		util.StdoutInto(util.StderrInto(t.Context(), t.Output()), t.Output()),
		slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
	)
}

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
		assert.NoError(t, err)
	})
	rawChartmuseumURL, err := chartmuseum.Endpoint(ctx, dagger.ServiceEndpointOpts{Scheme: "chartmuseum"})
	require.NoError(t, err)
	chartmuseumURL, err := url.Parse(rawChartmuseumURL)
	chartmuseumURL.RawQuery = "insecure=1"
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
		assert.NoError(t, err)
	})
	rawRegistryURL, err := registry.Endpoint(ctx, dagger.ServiceEndpointOpts{Scheme: "oci"})
	require.NoError(t, err)
	registryURL, err := url.Parse(rawRegistryURL)
	require.NoError(t, err)
	return registryURL
}
