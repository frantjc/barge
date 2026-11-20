//go:build dagger

package barge_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/frantjc/barge"
	"github.com/stretchr/testify/require"
)

func TestAuthChartmuseum(t *testing.T) {
	dag := Dag(t)
	c, archive := Archive(t)
	cm := Chartmuseum(t, dag, "BASIC_AUTH_USER", authUser, "BASIC_AUTH_PASS", authPassword)
	// After starting ChartMuseum, which may use the developer's credentials to
	// pull its image.
	IsolateCredentials(t)
	ctx := Context(t)

	require.ErrorContains(t, barge.Copy(ctx, archive.String(), cm.String()), "401")

	t.Setenv("BARGE_CHARTMUSEUM_DESTINATION_USERNAME", authUser)
	t.Setenv("BARGE_CHARTMUSEUM_DESTINATION_PASSWORD", authPassword)
	require.NoError(t, barge.Copy(ctx, archive.String(), cm.String()))

	repoURL := &url.URL{Scheme: "http", Host: cm.Host}
	chartURL := repoURL.JoinPath("charts", ChartFileName(c))
	require.ErrorContains(t, barge.Copy(ctx, chartURL.String(), t.TempDir()), "401")

	// With `helm repo add`'s credentials.
	repo := "chartmuseum"
	require.NoError(t, Command(t, "helm", "repo", "add", repo, repoURL.String(), "--username", authUser, "--password", authPassword).Run())
	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("repo://%s/%s?version=%s", repo, c.Name(), c.Metadata.Version), t.TempDir()))
	require.NoError(t, barge.Copy(ctx, chartURL.String(), t.TempDir()))
}
