package repo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
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

	chart := strings.TrimPrefix(u.Path, "/")
	version := u.Query().Get("version")

	chartEntry, ok := index.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart %s not found in repo %s", chart, u.Host)
	}

	chartVersion := xslices.Find(chartEntry, func(cv *repo.ChartVersion, _ int) bool {
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

	for _, rawURLOrPath := range chartVersion.URLs {
		if !strings.Contains(rawURLOrPath, "://") {
			w, err := url.Parse(entry.URL)
			if err != nil {
				return nil, err
			}
			rawURLOrPath = w.JoinPath(rawURLOrPath).String()
		}

		scheme, _, _ := strings.Cut(rawURLOrPath, "://")

		g, err := getter.All(settings).ByScheme(scheme)
		if err != nil {
			return nil, err
		}

		chartURL, err := url.Parse(rawURLOrPath)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		// Charts on the repository's host get its credentials. Charts
		// elsewhere only get credentials scoped to their own host, so that an
		// index can't collect the repository's.
		var cred *credential.Credential
		if repoURL, err := url.Parse(entry.URL); err == nil && repoURL.Host == chartURL.Host {
			cred, _ = credential.ForURL(ctx, chartURL, credential.Source, scheme)
		} else {
			cred, _ = credential.ForReferencedURL(ctx, chartURL, credential.Source)
		}

		buf, err := credential.Get(ctx, g, chartURL.String(), cred)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		return loader.LoadArchive(buf)
	}

	return nil, fmt.Errorf("could not get chart from urls: %w", errs)
}

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]barge.SyncableVersion, error) {
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

	versions, ok := index.Entries[name]
	if !ok {
		return nil, fmt.Errorf("chart %s not found in repo %s", name, u.Host)
	}

	res := []barge.SyncableVersion{}
	for _, chartVersion := range versions {
		version, err := semver.NewVersion(chartVersion.Version)
		if err != nil {
			continue
		}

		t := u.JoinPath(name)
		q := t.Query()
		q.Set("version", chartVersion.Version)
		t.RawQuery = q.Encode()

		w := barge.URL(*t)
		v := barge.Version(*version)

		res = append(res, barge.SyncableVersion{
			URL:     &w,
			Version: &v,
		})
	}

	return res, nil
}
