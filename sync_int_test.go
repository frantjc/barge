//go:build integration

package barge_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/http"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestSyncHTTP(t *testing.T) {
	ctx := Context(t)

	tmp, err := os.CreateTemp(t.TempDir(), "http.yml")
	require.NoError(t, err)

	constraints, err := semver.NewConstraint("3.9.0")
	require.NoError(t, err)

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
	require.NoError(t, err)
	_, err = tmp.Write(b)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())

	require.NoError(t, barge.Sync(ctx, tmp.Name(), t.TempDir()))
}
