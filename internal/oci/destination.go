package oci

import (
	"context"
	"io"
	"net/url"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/registry"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"oci",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	r, err := utils.NewRegistryClientFromURL(u)
	if err != nil {
		return err
	}

	rc, err := utils.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	ref := utils.RefFromURL(u)

	if _, err := r.Push(data, ref, registry.PushOptStrictMode(false)); err != nil {
		return err
	}

	return nil
}
