package artifactory

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
)

const (
	Scheme = "artifactory"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		Scheme,
		"rt",
		"jfrog",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	panic("unimplemented")
}
