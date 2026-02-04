package barge_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func FuzzSync(f *testing.F) {
	ctx := Context(f)

	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", tmp.Name()))
	require.NoError(f, err)

	tmp, err = os.CreateTemp(f.TempDir(), "archive-sync-config.yml")
	require.NoError(f, err)

	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
		},
	})
	require.NoError(f, err)
	_, err = tmp.Write(b)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	archiveSyncCfg := tmp.Name()

	f.Add(archiveSyncCfg, os.TempDir())

	if dag, err := dagger.Connect(ctx); assert.NoError(f, err) {
		f.Cleanup(func() {
			assert.NoError(f, dag.Close())
		})

		chartmuseumURL := Chartmuseum(f, dag)

		if helm, err := exec.LookPath("helm"); assert.NoError(f, err) {
			repo := "chartmuseum"
			repoURL := url.URL{
				Scheme: "http",
				Host:   chartmuseumURL.Host,
			}
			add := Command(f, helm, "repo", "add", repo, repoURL.String())
			require.NoError(f, add.Run())
		}

		f.Add(archiveSyncCfg, chartmuseumURL.String())

		registryURL := Registry(f, dag)
		f.Add(archiveSyncCfg, registryURL.String())
	}

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		require.NoError(t, barge.Sync(ctx, cfg, dest))
	})
}

func FuzzSyncError(f *testing.F) {
	ctx := Context(f)

	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", tmp.Name()))
	require.NoError(f, err)

	tmp, err = os.CreateTemp(f.TempDir(), "archive-sync-config.yml")
	require.NoError(f, err)

	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
		},
	})
	require.NoError(f, err)
	_, err = tmp.Write(b)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	cfg := tmp.Name()

	f.Add(cfg, "invalid://")
	f.Add(cfg, "oci://does-not-exist")

	if dag, err := dagger.Connect(ctx); assert.NoError(f, err) {
		f.Cleanup(func() {
			assert.NoError(f, dag.Close())
		})

		registryURL := Registry(f, dag)
		f.Add(filepath.Join(f.TempDir(), "does-not-exist.yaml"), registryURL.String())
	}

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		assert.Error(t, barge.Sync(ctx, cfg, dest))
	})
}
