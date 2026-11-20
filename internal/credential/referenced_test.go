package credential

import (
	"bytes"
	"log/slog"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/stretchr/testify/require"
)

func TestTryReferencedIgnoresEnvAndUserinfo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NETRC", filepath.Join(t.TempDir(), ".netrc"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("BARGE_HTTPS_SOURCE_PASSWORD", "env-pass")

	u, err := url.Parse("https://user:pass@charts.example.com/test-0.1.0.tgz")
	require.NoError(t, err)

	tried, err := record(func(fn func(*Credential) error) error {
		return TryReferenced(t.Context(), u, Source, fn)
	}, acceptAll)
	require.NoError(t, err)
	// Anonymously, with neither the userinfo nor the environment's.
	require.Equal(t, []*Credential{nil}, tried)
	require.Nil(t, u.User)
}

func TestTryReferencedIgnoresProviderQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NETRC", filepath.Join(t.TempDir(), ".netrc"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())

	buf := new(bytes.Buffer)
	ctx := util.SloggerInto(t.Context(), slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	u, err := url.Parse("https://evil.example.com/test-0.1.0.tgz?provider=azure")
	require.NoError(t, err)

	tried, err := record(func(fn func(*Credential) error) error {
		return TryReferenced(ctx, u, Source, fn)
	}, acceptAll)
	require.NoError(t, err)
	require.Equal(t, []*Credential{nil}, tried)
	// The cloud provider would log that it was skipped had it tried.
	require.NotContains(t, buf.String(), "provider=cloud")
	// The URL itself keeps its query.
	require.Equal(t, "azure", u.Query().Get("provider"))
}
