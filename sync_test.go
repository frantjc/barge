package barge_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"dagger.io/dagger"
	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/testdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
	"sigs.k8s.io/yaml"
)

func FuzzSyncOCI(f *testing.F) {
	ctx := Context(f)

	if dag, err := dagger.Connect(ctx); assert.NoError(f, err) {
		f.Cleanup(func() {
			assert.NoError(f, dag.Close())
		})

		tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
		require.NoError(f, err)
		_, err = tmp.Write(testdata.ChartArchive)
		require.NoError(f, err)
		require.NoError(f, tmp.Close())
		chart, err := loader.LoadFile(tmp.Name())
		require.NoError(f, err)
		archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", tmp.Name()))
		require.NoError(f, err)

		tmp, err = os.CreateTemp(f.TempDir(), "archive.yml")
		require.NoError(f, err)

		namespace := uuid.NewString()
		b, err := yaml.Marshal(&barge.SyncConfig{
			Sources: []barge.SourceConfig{
				{
					URL: barge.URL(*archiveURL),
				},
				{
					URL:       barge.URL(*archiveURL),
					Namespace: namespace,
				},
			},
		})
		require.NoError(f, err)
		_, err = tmp.Write(b)
		require.NoError(f, err)
		require.NoError(f, tmp.Close())
		archiveSyncCfg := tmp.Name()

		registryURL := Registry(f, dag)
		f.Add(archiveSyncCfg, registryURL.String())

		tmp, err = os.CreateTemp(f.TempDir(), "oci.yml")
		require.NoError(f, err)

		constraints, err := semver.NewConstraint(chart.Metadata.Version)
		require.NoError(f, err)

		b, err = yaml.Marshal(&barge.SyncConfig{
			Sources: []barge.SourceConfig{
				{
					URL: barge.URL(*registryURL),
					Charts: map[string]barge.Constraints{
						chart.Name(): barge.Constraints(*constraints),
					},
				},
				{
					URL: barge.URL(*registryURL.JoinPath(namespace)),
					Charts: map[string]barge.Constraints{
						chart.Name(): barge.Constraints(*constraints),
					},
				},
			},
		})
		require.NoError(f, err)
		_, err = tmp.Write(b)
		require.NoError(f, err)
		require.NoError(f, tmp.Close())
		ociSyncConfig := tmp.Name()
		f.Add(ociSyncConfig, f.TempDir())
	}

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		require.NoError(t, barge.Sync(ctx, cfg, dest))
	})
}

func FuzzSyncChartmuseum(f *testing.F) {
	ctx := Context(f)

	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", tmp.Name()))
	require.NoError(f, err)

	tmp, err = os.CreateTemp(f.TempDir(), "archive.yml")
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

	f.Add(archiveSyncCfg, f.TempDir())

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
	}

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		require.NoError(t, barge.Sync(ctx, cfg, dest))
	})
}

func FuzzSyncHTTP(f *testing.F) {
	ctx := Context(f)

	tmp, err := os.CreateTemp(f.TempDir(), "http.yml")
	require.NoError(f, err)

	constraints, err := semver.NewConstraint("3.9.0")
	require.NoError(f, err)

	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(url.URL{
					Scheme: "https",
					Host:   "chartmuseum.github.io",
					Path:   "/charts",
				}),
				Charts: map[string]barge.Constraints{
					"chartmuseum": barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(f, err)
	_, err = tmp.Write(b)
	require.NoError(f, err)
	require.NoError(f, tmp.Close())
	httpSyncCfg := tmp.Name()

	f.Add(httpSyncCfg, f.TempDir())

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		require.NoError(t, barge.Sync(ctx, cfg, dest))
	})
}

func FuzzSyncDirectory(f *testing.F) {
	ctx := Context(f)

	chartTgz, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = chartTgz.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, chartTgz.Close())
	chart, err := loader.LoadFile(chartTgz.Name())
	require.NoError(f, err)
	archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", chartTgz.Name()))
	require.NoError(f, err)

	archiveSyncCfg, err := os.CreateTemp(f.TempDir(), "archive.yml")
	require.NoError(f, err)

	namespace := uuid.NewString()
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
			{
				URL:       barge.URL(*archiveURL),
				Namespace: namespace,
			},
		},
	})
	require.NoError(f, err)
	_, err = archiveSyncCfg.Write(b)
	require.NoError(f, err)
	require.NoError(f, archiveSyncCfg.Close())
	archiveSyncCfgPath := archiveSyncCfg.Name()

	directoryURL, err := url.Parse(fmt.Sprintf("directory://%s", f.TempDir()))
	require.NoError(f, err)

	f.Add(archiveSyncCfgPath, directoryURL.String())

	directorySyncCfg, err := os.CreateTemp(f.TempDir(), "directory.yml")
	require.NoError(f, err)

	constraints, err := semver.NewConstraint(chart.Metadata.Version)
	require.NoError(f, err)

	b, err = yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*directoryURL),
				Charts: map[string]barge.Constraints{
					chart.Name(): barge.Constraints(*constraints),
				},
			},
			{
				URL:       barge.URL(*directoryURL.JoinPath(namespace)),
				Namespace: namespace,
				Charts: map[string]barge.Constraints{
					chart.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(f, err)
	_, err = directorySyncCfg.Write(b)
	require.NoError(f, err)
	require.NoError(f, directorySyncCfg.Close())
	directorySyncCfgPath := directorySyncCfg.Name()

	f.Add(directorySyncCfgPath, f.TempDir())

	f.Fuzz(func(t *testing.T, cfg, dest string) {
		require.NoError(t, barge.Sync(ctx, cfg, dest))
	})
}

func FuzzSyncFile(f *testing.F) {
	ctx := Context(f)

	chartTgz, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	require.NoError(f, err)
	_, err = chartTgz.Write(testdata.ChartArchive)
	require.NoError(f, err)
	require.NoError(f, chartTgz.Close())
	chart, err := loader.LoadFile(chartTgz.Name())
	require.NoError(f, err)
	archiveURL, err := url.Parse(fmt.Sprintf("archive://%s", chartTgz.Name()))
	require.NoError(f, err)

	archiveSyncCfg, err := os.CreateTemp(f.TempDir(), "archive.yml")
	require.NoError(f, err)

	namespace := uuid.NewString()
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
			{
				URL:       barge.URL(*archiveURL),
				Namespace: namespace,
			},
		},
	})
	require.NoError(f, err)
	_, err = archiveSyncCfg.Write(b)
	require.NoError(f, err)
	require.NoError(f, archiveSyncCfg.Close())
	archiveSyncCfgPath := archiveSyncCfg.Name()

	fileURL, err := url.Parse(fmt.Sprintf("file://%s", f.TempDir()))
	require.NoError(f, err)

	f.Add(archiveSyncCfgPath, fileURL.String())

	fileSyncCfg, err := os.CreateTemp(f.TempDir(), "file")
	require.NoError(f, err)

	constraints, err := semver.NewConstraint(chart.Metadata.Version)
	require.NoError(f, err)

	b, err = yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*fileURL),
				Charts: map[string]barge.Constraints{
					chart.Name(): barge.Constraints(*constraints),
				},
			},
			{
				URL:       barge.URL(*fileURL.JoinPath(namespace)),
				Namespace: namespace,
				Charts: map[string]barge.Constraints{
					chart.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(f, err)
	_, err = fileSyncCfg.Write(b)
	require.NoError(f, err)
	require.NoError(f, fileSyncCfg.Close())
	fileSyncCfgPath := fileSyncCfg.Name()

	f.Add(fileSyncCfgPath, f.TempDir())

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

	tmp, err = os.CreateTemp(f.TempDir(), "barge-sync.yml")
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
