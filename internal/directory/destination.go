package directory

import (
	"context"
	"net/url"
	"path/filepath"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"directory",
		"dir",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	return utils.WriteChartToDirectory(c, filepath.Join(u.Host, u.Path))	
}
