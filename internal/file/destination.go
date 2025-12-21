package file

import (
	"context"
	"net/url"
	"os"
	"path/filepath"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"file",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	fi, err := os.Stat(filepath.Join(u.Host, u.Path))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if fi.IsDir() {
		return utils.WriteChartToDirectory(c, fi.Name())
	}

	return utils.WriteChartToFile(c, filepath.Join(u.Host, u.Path))
}
