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
	name := filepath.Join(u.Host, u.Path)

	if fi, err := os.Stat(name); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if fi.IsDir() {
		return utils.WriteChartToDirectory(c, name)
	}

	return utils.WriteChartToFile(c, name)
}
