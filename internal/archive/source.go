package archive

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
		"archive",
		"tar",
		"tarball",
		"package",
		"pkg",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	c, err := loader.LoadFile(filepath.Join(u.Host, u.Path))
	if err != nil {
		return nil, err
	}

	return c, nil
}

