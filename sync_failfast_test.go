package barge_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// With FailFast, charts must still sync after their sources have been listed,
// rather than be canceled once listing finishes.
func TestSyncFailFast(t *testing.T) {
	t.Parallel()
	c := testChart(t)
	srv := NewAuthServer(t, ChartRepoHandler(t, c, ChartFileName(c)), nil)

	constraints, err := semver.NewConstraint(c.Metadata.Version)
	require.NoError(t, err)
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{
				URL: barge.URL(*srv.URLWith(t, "http", "", nil)),
				Charts: map[string]barge.Constraints{
					c.Name(): barge.Constraints(*constraints),
				},
			},
		},
	})
	require.NoError(t, err)
	cfg := filepath.Join(t.TempDir(), "sync.yml")
	require.NoError(t, os.WriteFile(cfg, b, 0o600))

	dest := t.TempDir()
	require.NoError(t, barge.Sync(Context(t), cfg, dest, barge.WithFailFast()))
	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}

func TestSyncFailFastReturnsError(t *testing.T) {
	t.Parallel()
	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{
			{URL: barge.URL{Scheme: "archive", Path: filepath.Join(t.TempDir(), "missing.tgz")}},
		},
	})
	require.NoError(t, err)
	cfg := filepath.Join(t.TempDir(), "sync.yml")
	require.NoError(t, os.WriteFile(cfg, b, 0o600))

	require.Error(t, barge.Sync(Context(t), cfg, t.TempDir(), barge.WithFailFast()))
}
