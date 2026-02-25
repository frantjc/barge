package http

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
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
	log := util.SloggerFrom(ctx)
	settings := barge.HelmSettings()
	scheme := u.Scheme

	g, err := getter.All(settings).ByScheme(scheme)
	if err != nil {
		return nil, err
	}

	opts := []getter.Option{}

	if username, password, ok := util.UsernameAndPasswordForURLWithEnvFallback(u, util.LocationSource, scheme); ok {
		opts = append(opts, getter.WithBasicAuth(username, password))
	}

	w := u.JoinPath()
	name := path.Base(w.Path)
	log.Info("opening", "url", w.String(), "ext", path.Ext(name))
	if version := u.Query().Get("version"); version != "" {
		if ext := path.Ext(name); ext == "" {
			w.Path = path.Join("/", path.Dir(u.Path), "index.yaml")

			buf, err := g.Get(w.String(), opts...)
			if err != nil {
				return nil, err
			}

			index := repo.NewIndexFile()
			if err := yaml.Unmarshal(buf.Bytes(), index); err != nil {
				return nil, err
			}

			chartEntry, ok := index.Entries[name]
			if !ok {
				return nil, fmt.Errorf("chart %s not found in repo %s", name, u.String())
			}

			for _, chartVersion := range chartEntry {
				if chartVersion.Version == version {

					var errs error

					for _, rawURLOrPath := range chartVersion.URLs {
						if !strings.Contains(rawURLOrPath, "://") {
							w, err := url.Parse(rawURLOrPath)
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

						opts := []getter.Option{}

						if username, password, ok := util.UsernameAndPasswordForURLWithEnvFallback(u, util.LocationSource, scheme); ok {
							opts = append(opts, getter.WithBasicAuth(username, password))
						}

						buf, err := g.Get(rawURLOrPath, opts...)
						if err != nil {
							errs = errors.Join(errs, err)
							continue
						}

						return loader.LoadArchive(buf)
					}

					return nil, fmt.Errorf("could not get chart from urls: %w", errs)
				}
			}
		}
	}

	buf, err := g.Get(w.String(), opts...)
	if err != nil {
		return nil, err
	}

	return loader.LoadArchive(buf)
}

func (s *source) Versions(ctx context.Context, u *url.URL, name string) ([]barge.SyncableVersion, error) {
	settings := barge.HelmSettings()
	scheme := u.Scheme

	g, err := getter.All(settings).ByScheme(scheme)
	if err != nil {
		return nil, err
	}

	opts := []getter.Option{}

	if username, password, ok := util.UsernameAndPasswordForURLWithEnvFallback(u, util.LocationSource, scheme); ok {
		opts = append(opts, getter.WithBasicAuth(username, password))
	}

	buf, err := g.Get(u.JoinPath("index.yaml").String(), opts...)
	if err != nil {
		return nil, err
	}

	index := repo.NewIndexFile()
	if err := yaml.Unmarshal(buf.Bytes(), index); err != nil {
		return nil, err
	}

	versions, ok := index.Entries[name]
	if !ok {
		return nil, fmt.Errorf("chart %s not found in repo %s", name, u.String())
	}

	res := []barge.SyncableVersion{}
	for _, chartVersion := range versions {
		version, err := semver.NewVersion(chartVersion.Version)
		if err != nil {
			continue
		}

		for _, rawURLOrPath := range chartVersion.URLs {
			t, err := url.Parse(rawURLOrPath)
			if err != nil {
				continue
			}

			if !t.IsAbs() {
				t = u.ResolveReference(t)
			}

			w := barge.URL(*t)
			v := barge.Version(*version)

			res = append(res, barge.SyncableVersion{
				URL:     &w,
				Version: &v,
			})
		}
	}

	return res, nil
}
