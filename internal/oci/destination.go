package oci

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/registry"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"oci",
		"oci+https",
		"oci+http",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	r, err := newRegistryClient(ctx, u, credential.Destination)
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

	// Like `helm push`, tag the chart with its version unless the URL says
	// otherwise, via a tag in its path or its version query parameter.
	w := *u
	if q := w.Query(); !strings.Contains(w.Path, ":") && q.Get("version") == "" {
		q.Set("version", c.Metadata.Version)
		w.RawQuery = q.Encode()
	}

	ref := util.RefFromURL(&w)

	if _, err := r.Push(data, ref, registry.PushOptStrictMode(false)); err != nil {
		return err
	}

	return nil
}

func (d *destination) Sync(ctx context.Context, u *url.URL, namespace, name string, c *chart.Chart) error {
	if name == "" {
		name = c.Name()
	}
	v := u.JoinPath()
	v.Path, _, _ = strings.Cut(v.Path, ":")
	q := v.Query()
	q.Set("version", c.Metadata.Version)
	v.RawQuery = q.Encode()
	return d.Write(ctx, v.JoinPath(namespace, name), c)
}
