//go:build dagger

package barge_test

import (
	"os"
	"path/filepath"
	"testing"

	"dagger.io/dagger"
	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestSyncDistribution(t *testing.T) {
	ctx := Context(t)

	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})

	archiveChart, archiveURL := Archive(t)

	namespace := uuid.NewString()
	distrubutionURL := Distrubition(t, dag)

	// Sync from archive into OCI registry.
	archiveSyncCfg, err := os.CreateTemp(t.TempDir(), "archive.yml")
	require.NoError(t, err)
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
	require.NoError(t, err)
	_, err = archiveSyncCfg.Write(b)
	require.NoError(t, err)
	require.NoError(t, archiveSyncCfg.Close())

	require.NoError(t, barge.Sync(ctx, archiveSyncCfg.Name(), distrubutionURL.String()))

	// Sync from OCI registry into a directory.
	constraints, err := semver.NewConstraint(archiveChart.Metadata.Version)
	require.NoError(t, err)

	ociSyncCfg, err := os.CreateTemp(t.TempDir(), "oci.yml")
	require.NoError(t, err)
	b, err = yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*distrubutionURL),
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
			{
				URL: barge.URL(*distrubutionURL.JoinPath(namespace)),
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = ociSyncCfg.Write(b)
	require.NoError(t, err)
	require.NoError(t, ociSyncCfg.Close())

	require.NoError(t, barge.Sync(ctx, ociSyncCfg.Name(), t.TempDir()))
}

func TestSyncChartmuseum(t *testing.T) {
	ctx := Context(t)

	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})

	_, archiveURL := Archive(t)

	archiveSyncCfg, err := os.CreateTemp(t.TempDir(), "archive.yml")
	require.NoError(t, err)
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
		},
	})
	require.NoError(t, err)
	_, err = archiveSyncCfg.Write(b)
	require.NoError(t, err)
	require.NoError(t, archiveSyncCfg.Close())

	chartmuseumURL := Chartmuseum(t, dag)

	require.NoError(t, barge.Sync(ctx, archiveSyncCfg.Name(), chartmuseumURL.String()))
}

func TestSyncErrorMissingConfig(t *testing.T) {
	ctx := Context(t)

	dag, err := dagger.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dag.Close())
	})

	distrubutionURL := Distrubition(t, dag)
	require.Error(t, barge.Sync(ctx, filepath.Join(t.TempDir(), "does-not-exist.yaml"), distrubutionURL.String()))
}
