//go:build kubernetes

package barge_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/release"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCopyRelease(t *testing.T) {
	ctx := Context(t)

	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	require.NoError(t, err)

	cli, err := kubernetes.NewForConfig(restCfg)
	require.NoError(t, err)

	_, archiveURL := Archive(t)
	chart := filepath.Join(archiveURL.Host, archiveURL.Path)
	release := "r" + uuid.NewString()
	namespace := "r" + uuid.NewString()

	create := Command(t, "helm", "upgrade", "--install", release, chart, "--namespace", namespace, "--create-namespace")
	require.NoError(t, create.Run())
	t.Cleanup(func() {
		require.NoError(t, cli.CoreV1().Namespaces().Delete(context.WithoutCancel(ctx), namespace, metav1.DeleteOptions{}))
	})

	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("release://%s?namespace=%s", release, namespace), t.TempDir()))
}
