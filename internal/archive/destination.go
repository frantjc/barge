package archive

import (
	"context"
	"net/url"
	"path/filepath"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"archive",
		"tar",
		"tarball",
		"package",
		"pkg",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	return util.WriteChartToFile(c, filepath.Join(u.Host, u.Path))
}
