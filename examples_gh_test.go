//go:build examples && github

package barge_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/require"
)

func TestExampleHTTP(t *testing.T) {
	ctx := Context(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testdata.ChartArchive)
	}))
	t.Cleanup(srv.Close)
	require.NoError(t, barge.Copy(ctx, "https://github.com/frantjc/barge/raw/refs/heads/main/testdata/test-0.1.0.tgz", t.TempDir()))
}

func TestExampleSync(t *testing.T) {
	ctx := Context(t)
	repoName := "chartmuseum"
	add := Command(t, "helm", "repo", "add", "--force-update", repoName, "https://chartmuseum.github.io/charts")
	t.Cleanup(func() {
		remove := Command(t, "helm", "repo", "remove", repoName)
		require.NoError(t, remove.Run())
	})
	require.NoError(t, add.Run())
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	require.NoError(t, barge.Sync(ctx, filepath.Join(filepath.Dir(file), "barge-sync.yml"), t.TempDir(), barge.WithFailFast()))
}
