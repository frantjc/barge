package barge_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"testing"

	"dagger.io/dagger"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/internal/util"
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
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
	return util.StdoutInto(util.StderrInto(t.Context(), t.Output()), t.Output())
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

func FuzzCopy(f *testing.F) {
	ctx := Context(f)

	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	chart, err := loader.LoadFile(tmp.Name())
	require.NoError(f, err)
	archive := fmt.Sprintf("archive://%s", tmp.Name())
	directory := fmt.Sprintf("directory://%s", f.TempDir())
	f.Add(archive, directory)
	f.Add(directory, f.TempDir())

	file := f.TempDir()
	f.Add(archive, file)
	f.Add(file, f.TempDir())

	if dag, err := dagger.Connect(ctx); assert.NoError(f, err) {
		f.Cleanup(func() {
			assert.NoError(f, dag.Close())
		})

		chartmuseumURL := Chartmuseum(f, dag)
		require.NoError(f, barge.Copy(ctx, archive, chartmuseumURL.String()))

		if helm, err := exec.LookPath("helm"); assert.NoError(f, err) {
			repo := "chartmuseum"
			repoURL := url.URL{
				Scheme: "http",
				Host:   chartmuseumURL.Host,
			}
			add := Command(f, helm, "repo", "add", repo, repoURL.String())
			require.NoError(f, add.Run())

			f.Add(fmt.Sprintf("repo://%s/%s", repo, chart.Name()), f.TempDir())
			f.Add(fmt.Sprintf("repo://%s/%s?version=%s", repo, chart.Name(), chart.Metadata.Version), f.TempDir())
		}

		httpURL := &url.URL{
			Scheme: "http",
			Host:   chartmuseumURL.Host,
			Path:   chartmuseumURL.JoinPath("charts/test-0.1.0.tgz").Path,
		}
		f.Add(httpURL.String(), f.TempDir())

		registryURL := Registry(f, dag)
		oci := registryURL.JoinPath("test")
		f.Add(archive, oci.String())
		f.Add(oci.String(), f.TempDir())
		ociWithTag := registryURL.JoinPath("test:tag")
		f.Add(archive, ociWithTag.String())
		f.Add(ociWithTag.String(), f.TempDir())
	}

	if githubRepository := os.Getenv("GITHUB_REPOSITORY"); githubRepository != "" {
		ghcr := fmt.Sprintf("oci://ghcr.io/%s/charts/%s", githubRepository, chart.Name())
		ghcrWithTag := fmt.Sprintf("%s:%s", ghcr, chart.Metadata.Version)
		f.Add(archive, ghcr)
		f.Add(ghcr, f.TempDir())
		f.Add(archive, ghcrWithTag)
		f.Add(ghcrWithTag, f.TempDir())
	}

	f.Fuzz(func(t *testing.T, src, dest string) {
		require.NoError(t, barge.Copy(t.Context(), src, dest))
	})
}

func FuzzCopyError(f *testing.F) {
	f.Add("foo://", f.TempDir())
	f.Add(f.TempDir(), "bar://")

	f.Fuzz(func(t *testing.T, src, dest string) {
		require.Error(t, barge.Copy(t.Context(), src, dest))
	})
}
