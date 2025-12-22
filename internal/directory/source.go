package directory

import (
	"context"
	"net/url"
	"path/filepath"

	"github.com/frantjc/barge"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"directory",
		"dir",
		"source",
		"src",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	return loader.LoadDir(filepath.Join(u.Host, u.Path))
}
