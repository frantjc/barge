package file

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
		return util.WriteChartToDirectory(ctx, c, name)
	}

	return util.WriteChartToFile(c, name)
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace string, c *chart.Chart) error {
	name := filepath.Join(u.Host, u.Path)

	if fi, err := os.Stat(name); err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("cannot sync to a file; try a directory")
	}

	target := filepath.Join(name, namespace, fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version))
	if namespace != "" {
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
	}

	return util.WriteChartToFile(c, target)
}
