package barge_test

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestSyncDirectory(t *testing.T) {
	ctx := Context(t)

	archiveChart, archive := Archive(t)
	archiveURL, err := url.Parse(archive)
	require.NoError(t, err)

	namespace := uuid.NewString()

	// Sync from archive into directory.
	directoryURL, err := url.Parse(fmt.Sprintf("directory://%s", t.TempDir()))
	require.NoError(t, err)

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

	require.NoError(t, barge.Sync(ctx, archiveSyncCfg.Name(), directoryURL.String()))

	// Sync from directory into a new directory.
	constraints, err := semver.NewConstraint(archiveChart.Metadata.Version)
	require.NoError(t, err)

	directorySyncCfg, err := os.CreateTemp(t.TempDir(), "directory.yml")
	require.NoError(t, err)
	b, err = yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*directoryURL),
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
			{
				URL:       barge.URL(*directoryURL.JoinPath(namespace)),
				Namespace: namespace,
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = directorySyncCfg.Write(b)
	require.NoError(t, err)
	require.NoError(t, directorySyncCfg.Close())

	require.NoError(t, barge.Sync(ctx, directorySyncCfg.Name(), t.TempDir()))
}

func TestSyncFile(t *testing.T) {
	ctx := Context(t)

	archiveChart, archive := Archive(t)
	archiveURL, err := url.Parse(archive)
	require.NoError(t, err)

	namespace := uuid.NewString()

	// Sync from archive into file destination.
	fileURL, err := url.Parse(fmt.Sprintf("file://%s", t.TempDir()))
	require.NoError(t, err)

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

	require.NoError(t, barge.Sync(ctx, archiveSyncCfg.Name(), fileURL.String()))

	// Sync from file source into a new directory.
	constraints, err := semver.NewConstraint(archiveChart.Metadata.Version)
	require.NoError(t, err)

	fileSyncCfg, err := os.CreateTemp(t.TempDir(), "file.yml")
	require.NoError(t, err)
	b, err = yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*fileURL),
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
			{
				URL:       barge.URL(*fileURL.JoinPath(namespace)),
				Namespace: namespace,
				Charts: map[string]barge.Constraints{
					archiveChart.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = fileSyncCfg.Write(b)
	require.NoError(t, err)
	require.NoError(t, fileSyncCfg.Close())

	require.NoError(t, barge.Sync(ctx, fileSyncCfg.Name(), t.TempDir()))
}

func TestSyncErrorInvalidDest(t *testing.T) {
	ctx := Context(t)

	_, archive := Archive(t)
	archiveURL, err := url.Parse(archive)
	require.NoError(t, err)

	cfgFile, err := os.CreateTemp(t.TempDir(), "barge-sync.yml")
	require.NoError(t, err)
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*archiveURL),
			},
		},
	})
	require.NoError(t, err)
	_, err = cfgFile.Write(b)
	require.NoError(t, err)
	require.NoError(t, cfgFile.Close())
	cfg := cfgFile.Name()

	require.Error(t, barge.Sync(ctx, cfg, "invalid://"))
	require.Error(t, barge.Sync(ctx, cfg, "oci://does-not-exist"))
}
