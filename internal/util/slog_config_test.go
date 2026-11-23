package util_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/frantjc/barge/internal/util"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestSlogConfigAddFlags(t *testing.T) {
	var (
		slogConfig = new(util.SlogConfig)
		flagSet    = pflag.NewFlagSet("test", pflag.ContinueOnError)
	)

	require.NoError(t, os.Unsetenv("DEBUG"))

	slogConfig.AddFlags(flagSet)

	require.Equal(t, slog.LevelInfo, slogConfig.Level())

	require.NoError(t, flagSet.Parse([]string{"--debug"}))
	require.Equal(t, slog.LevelDebug, slogConfig.Level())

	require.NoError(t, flagSet.Parse([]string{"--quiet"}))
	require.Equal(t, slog.LevelError, slogConfig.Level())

	require.NoError(t, flagSet.Parse([]string{"-v"}))
	require.Equal(t, slog.LevelWarn, slogConfig.Level())
}
