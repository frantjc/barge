package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/artifactory"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/release"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/internal/util"
	xerrors "github.com/frantjc/x/errors"
	xos "github.com/frantjc/x/os"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	err := xerrors.Ignore(newBarge().ExecuteContext(ctx), context.Canceled)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	stop()
	xos.ExitFromError(err)
}

func newBarge() *cobra.Command {
	var (
		slogConfig = new(util.SlogConfig)
		cmd        = &cobra.Command{
			Use:           "barge",
			Version:       SemVer(),
			SilenceErrors: true,
			SilenceUsage:  true,
			PersistentPreRun: func(cmd *cobra.Command, args []string) {
				cmd.SetContext(
					util.SloggerInto(
						util.StdoutInto(
							util.StderrInto(
								cmd.Context(),
								cmd.ErrOrStderr(),
							),
							cmd.OutOrStdout(),
						),
						slog.New(slog.NewJSONHandler(cmd.OutOrStdout(), &slog.HandlerOptions{
							Level: slogConfig,
						})),
					),
				)
			},
		}
	)
	cmd.Flags().BoolP("help", "h", false, "Help for "+cmd.Name())
	cmd.Flags().Bool("version", false, "Version for "+cmd.Name())
	cmd.SetVersionTemplate("{{ .Name }}{{ .Version }}")
	slogConfig.AddFlags(cmd.PersistentFlags())
	cmd.AddCommand(newCopy())
	return cmd
}

func newCopy() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "copy",
		Aliases:       []string{"cp"},
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return barge.Copy(cmd.Context(), args[0], args[1])
		},
	}
	barge.AddFlags(cmd.Flags())
	return cmd
}
