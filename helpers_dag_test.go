//go:build dagger

package barge_test

import (
	"context"
	"crypto/rand"
	"net"
	"net/url"
	"testing"

	"dagger.io/dagger"
	"dagger.io/dagger/core"
	"github.com/frantjc/barge/internal/credential"
	"github.com/stretchr/testify/require"
)

// Chartmuseum starts ChartMuseum, configured by env, pairs of environment
// variable names and values, e.g. "BASIC_AUTH_USER", "barge".
func Chartmuseum(t testing.TB, dag *core.Query, env ...string) *url.URL {
	t.Helper()
	require.Zero(t, len(env)%2, "env must be name-value pairs")
	ctx := t.Context()
	container := dag.Container()
	if cred, err := (credential.GitHub{}).Credential(ctx, &credential.Request{URL: &url.URL{Scheme: "https", Host: "ghcr.io"}}); err == nil && cred != nil {
		container = container.WithRegistryAuth("ghcr.io", cred.Username, dag.SetSecret("github-password", cred.Password))
	}
	chartmuseum, err := container.
		From("ghcr.io/helm/chartmuseum:v0.16.5").
		WithExposedPort(8080).
		WithEnvVariable("CACHEBUSTER", rand.Text()).
		WithEnvVariable("DEBUG", "1").
		WithEnvVariable("STORAGE", "local").
		WithEnvVariable("STORAGE_LOCAL_ROOTDIR", "/tmp").
		With(func(c *core.Container) *core.Container {
			for i := 0; i < len(env); i += 2 {
				c = c.WithEnvVariable(env[i], env[i+1])
			}
			return c
		}).
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = chartmuseum.Stop(Context(t))
		require.NoError(t, err)
	})
	rawChartmuseumURL := Endpoint(t, dag, chartmuseum, core.ServiceEndpointOpts{Scheme: "chartmuseum+http"})
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
		WithEnvVariable("CACHEBUSTER", rand.Text()).
		AsService().
		Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = distribution.Stop(Context(t))
		require.NoError(t, err)
	})
	rawDistributionURL := Endpoint(t, dag, distribution, core.ServiceEndpointOpts{Scheme: "oci+http"})
	distributionURL, err := url.Parse(rawDistributionURL)
	require.NoError(t, err)
	return distributionURL
}

func Dag(t testing.TB) *core.Query {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})
	return core.NewQuery(dag)
}

// Endpoint returns the host:port at which the Service can be reached both in and out of Dagger.
func Endpoint(t testing.TB, dag *core.Query, svc *core.Service, opts ...core.ServiceEndpointOpts) string {
	t.Helper()

	ctx := Context(t)

	hostname, err := svc.Hostname(ctx)
	require.NoError(t, err)

	if _, err := net.DefaultResolver.LookupHost(ctx, hostname); err != nil {
		svc, err = dag.Host().Tunnel(svc).Start(ctx)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, err := svc.Stop(Context(t))
			require.NoError(t, err)
		})
	}

	endpoint, err := svc.Endpoint(ctx, opts...)
	require.NoError(t, err)

	return endpoint
}
