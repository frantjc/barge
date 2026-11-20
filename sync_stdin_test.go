package barge_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// A config of "-" is read from the context's stdin.
func TestSyncConfigFromStdin(t *testing.T) {
	t.Parallel()
	_, archive := Archive(t)

	b, err := yaml.Marshal(&barge.SyncConfig{
		Sources: []barge.SourceConfig{{URL: barge.URL(*archive)}},
	})
	require.NoError(t, err)

	dest := t.TempDir()
	ctx := util.StdinInto(Context(t), bytes.NewReader(b))
	require.NoError(t, barge.Sync(ctx, "-", dest))

	entries, err := os.ReadDir(dest)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}
