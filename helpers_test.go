package barge_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func Archive(t testing.TB) (*chart.Chart, string) {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), "test-0.1.0.tgz")
	require.NoError(t, err)

	_, err = tmp.Write(testdata.ChartArchive)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())

	testChartURL := fmt.Sprintf("archive://%s", tmp.Name())
	testChart, err := loader.LoadFile(tmp.Name())
	require.NoError(t, err)

	require.NotNil(t, testChart)

	return testChart, testChartURL
}

func Command(t testing.TB, name string, arg ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, arg...)
	cmd.Stdout = t.Output()
	cmd.Stderr = t.Output()
	return cmd
}

func Context(t testing.TB) context.Context {
	t.Helper()
	return util.SloggerInto(
		util.StdoutInto(util.StderrInto(t.Context(), t.Output()), t.Output()),
		slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug})),
	)
}
