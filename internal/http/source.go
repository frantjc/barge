package http

import (
	"context"
	"net/http"
	"net/url"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"http",
		"https",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(resp.Body)
}
