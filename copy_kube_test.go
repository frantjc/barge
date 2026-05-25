//go:build kubernetes

package barge_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
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

	helm, err := exec.LookPath("helm")
	require.NoError(t, err)

	chart, archive := Archive(t)
	releaseName := chart.Name()
	namespace := uuid.NewString()

	create := Command(t, helm, "upgrade", "--install", releaseName, strings.TrimPrefix(archive, "archive://"), "--namespace", namespace, "--create-namespace")
	require.NoError(t, create.Run())
	t.Cleanup(func() {
		require.NoError(t, cli.CoreV1().Namespaces().Delete(context.WithoutCancel(ctx), namespace, metav1.DeleteOptions{}))
	})

	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("release://%s?namespace=%s", releaseName, namespace), t.TempDir()))
}
