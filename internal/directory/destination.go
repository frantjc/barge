package directory

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
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
	return util.WriteChartToDirectory(ctx, c, filepath.Join(u.Host, u.Path))
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace string, c *chart.Chart) error {
	root := filepath.Join(u.Host, u.Path)
	if fi, err := os.Stat(root); err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("cannot sync to a file; try a directory")
	}

	if namespace != "" {
		if err := os.MkdirAll(filepath.Join(root, namespace), 0755); err != nil {
			return err
		}
	}

	return util.WriteChartToFile(c, filepath.Join(root, fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version)))
}
