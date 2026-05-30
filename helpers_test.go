package barge_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/frantjc/barge/testdata"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"sigs.k8s.io/yaml"
)

func Archive(t testing.TB) (*chart.Chart, *url.URL) {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "test-0.1.0.tgz")
	require.NoError(t, err)
	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	chartURL, err := url.Parse(fmt.Sprintf("archive://%s", tmp.Name()))
	require.NoError(t, err)
	chart, err := loader.LoadFile(tmp.Name())
	require.NoError(t, err)
	require.NotNil(t, chart)
	return chart, chartURL
}

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

// Repo starts an in-process Helm chart repository serving the given chart,
// registers it with `helm repo add` using a random name, and returns the repo
// name and the server's base URL.
func Repo(t testing.TB, chart *chart.Chart) (string, *url.URL) {
	t.Helper()

	rootPath := "/"
	chartPath := filepath.Join(rootPath, fmt.Sprintf("%s-%s.tgz", chart.Name(), chart.Metadata.Version))
	chartRelPath, err := filepath.Rel(rootPath, chartPath)
	require.NoError(t, err)
	indexYAML, err := yaml.Marshal(map[string]any{
		"apiVersion": "v1",
		"entries": map[string]any{
			chart.Name(): []map[string]any{
				{
					"name":    chart.Name(),
					"version": chart.Metadata.Version,
					"urls":    []string{chartRelPath},
				},
			},
		},
	})
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			_, _ = w.Write(indexYAML)
		case chartPath:
			_, _ = w.Write(testdata.ChartArchive)
		}
	}))
	t.Cleanup(srv.Close)

	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	repoName := uuid.NewString()
	add := Command(t, "helm", "repo", "add", repoName, srv.URL)
	require.NoError(t, add.Run())
	t.Cleanup(func() {
		remove := exec.Command("helm", "repo", "remove", repoName)
		require.NoError(t, remove.Run())
	})

	return repoName, srvURL
}

// OCI starts an in-process OCI registry and returns its URL.
func OCI(t testing.TB) *url.URL {
	t.Helper()

	reg := httptest.NewServer(registry.New())
	t.Cleanup(reg.Close)

	return &url.URL{
		Scheme: "oci",
		Host:   reg.Listener.Addr().String(),
	}
}
