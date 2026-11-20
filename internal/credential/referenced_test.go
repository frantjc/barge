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

func TestForReferencedURLIgnoresEnvAndUserinfo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NETRC", filepath.Join(t.TempDir(), ".netrc"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("BARGE_HTTPS_SOURCE_PASSWORD", "env-pass")

	u, err := url.Parse("https://user:pass@charts.example.com/test-0.1.0.tgz")
	require.NoError(t, err)

	cred, ok := ForReferencedURL(t.Context(), u, Source)
	require.False(t, ok)
	require.Nil(t, cred)
	require.Nil(t, u.User)
}

func TestForReferencedURLIgnoresProviderQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NETRC", filepath.Join(t.TempDir(), ".netrc"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())

	buf := new(bytes.Buffer)
	ctx := util.SloggerInto(t.Context(), slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	u, err := url.Parse("https://evil.example.com/test-0.1.0.tgz?provider=azure")
	require.NoError(t, err)

	_, ok := ForReferencedURL(ctx, u, Source)
	require.False(t, ok)
	// The cloud provider would log that it was skipped had it tried.
	require.NotContains(t, buf.String(), "provider=cloud")
	// The URL itself keeps its query.
	require.Equal(t, "azure", u.Query().Get("provider"))
}
