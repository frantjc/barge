package chartmuseum

import (
	"context"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterSource(
		new(source),
		Scheme,
		"cm",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	panic("unimplemented")
}
