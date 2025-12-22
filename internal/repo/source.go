package repo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/frantjc/barge"
	xslices "github.com/frantjc/x/slices"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

func init() {
	barge.RegisterSource(
		new(source),
		"repo",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	settings := barge.HelmSettings()

	repos, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		return nil, err
	}

	entry := repos.Get(u.Host)
	if entry == nil {
		return nil, fmt.Errorf("unknown repo %s", u.Host)
	}

	index, err := repo.LoadIndexFile(
		filepath.Join(settings.RepositoryCache, helmpath.CacheIndexFile(u.Host)),
	)
	if err != nil {
		return nil, err
	}

	chart, version, _ := strings.Cut(
		strings.TrimPrefix(u.Path, "/"),
		"/",
	)

	versions, ok := index.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart %s not found in repo %s", chart, u.Host)
	}

	chartVersion := xslices.Find(versions, func(cv *repo.ChartVersion, _ int) bool {
		if version == "" {
			return true
		}
		return cv.Version == version
	})
	if chartVersion == nil {
		if version == "" {
			return nil, fmt.Errorf("no versions found for chart %s", chart)
		}

		return nil, fmt.Errorf("chart %s version %s not found", chart, version)
	} else if len(chartVersion.URLs) == 0 {
		return nil, fmt.Errorf("chart %s version %s has no urls", chart, chartVersion.Version)
	}

	var errs error

	for _, v := range chartVersion.URLs {
		if !strings.Contains(v, "://") {
			w, err := url.Parse(entry.URL)
			if err != nil {
				return nil, err
			}
			v = w.JoinPath(v).String()
		}

		scheme, _, _ := strings.Cut(v, "://")

		g, err := getter.All(settings).ByScheme(scheme)
		if err != nil {
			return nil, err
		}

		buf, err := g.Get(v)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		return loader.LoadArchive(buf)
	}

	return nil, fmt.Errorf("could not get chart from urls: %w", errs)
}
