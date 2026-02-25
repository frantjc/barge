package oci

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
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
	r, err := util.NewRegistryClientFromURL(ctx, u)
	if err != nil {
		return err
	}

	rc, err := util.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	ref := util.RefFromURL(u)

	if _, err := r.Push(data, ref, registry.PushOptStrictMode(false)); err != nil {
		return err
	}

	return nil
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace string, c *chart.Chart) error {
	v := u.JoinPath()
	v.Path, _, _ = strings.Cut(v.Path, ":")
	q := v.Query()
	q.Set("version", c.Metadata.Version)
	v.RawQuery = q.Encode()
	return d.Write(ctx, v.JoinPath(namespace, c.Name()), c)
}
