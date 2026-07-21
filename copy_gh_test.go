//go:build github

package barge_test

import (
	"fmt"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/git"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCopyGitHub(t *testing.T) {
	ctx := Context(t)
	git := "git://github.com/chartmuseum/charts/src/chartmuseum"
	tmp := t.TempDir()
	require.NoError(t, barge.Copy(ctx, git, tmp))
	require.NoError(t, barge.Copy(ctx, tmp, t.TempDir()))
}

func TestCopyRawGitHubUserContent(t *testing.T) {
	ctx := Context(t)
	repoName := uuid.NewString()
	add := Command(t, "helm", "repo", "add", repoName, "https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts")
	require.NoError(t, add.Run())
	t.Cleanup(func() {
		remove := Command(t, "helm", "repo", "remove", repoName)
		require.NoError(t, remove.Run())
	})
	repo := fmt.Sprintf("repo://%s/azuredisk-csi-driver", repoName)
	tmp := t.TempDir()
	require.NoError(t, barge.Copy(ctx, repo, tmp))
	require.NoError(t, barge.Copy(ctx, tmp, t.TempDir()))
}

func TestCopyGitHubIO(t *testing.T) {
	ctx := Context(t)
	repoName := uuid.NewString()
	add := Command(t, "helm", "repo", "add", repoName, "https://coredns.github.io/helm")
	require.NoError(t, add.Run())
	t.Cleanup(func() {
		remove := Command(t, "helm", "repo", "remove", repoName)
		require.NoError(t, remove.Run())
	})
	repo := fmt.Sprintf("repo://%s/coredns", repoName)
	tmp := t.TempDir()
	require.NoError(t, barge.Copy(ctx, repo, tmp))
	require.NoError(t, barge.Copy(ctx, tmp, t.TempDir()))
}
