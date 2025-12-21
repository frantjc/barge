package chartmuseum

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
)

const (
	Scheme = "chartmuseum"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		Scheme,
		"cm",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	panic("unimplemented")
}
