package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/credential"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
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
	settings := barge.HelmSettings()
	scheme := u.Scheme

	g, err := getter.All(settings).ByScheme(scheme)
	if err != nil {
		return nil, err
	}

	var c *chart.Chart
	err = credential.Try(ctx, u, credential.Source, []string{scheme}, func(cred *credential.Credential) error {
		var err error
		c, err = s.open(ctx, settings, g, u, cred)
		return err
	})

	return c, err
}

// open opens the chart at u, authenticating with cred. A rejection of cred by
// u's host is returned as is, for credential.Try to move on to the next
// credentials.
func (s *source) open(ctx context.Context, settings *cli.EnvSettings, g getter.Getter, u *url.URL, cred *credential.Credential) (*chart.Chart, error) {
	w := u.JoinPath()
	name := path.Base(w.Path)
	if version := u.Query().Get("version"); version != "" {
		if ext := path.Ext(name); ext == "" {
			w.Path = path.Join("/", path.Dir(u.Path), "index.yaml")

			buf, err := credential.Get(ctx, g, w.String(), cred)
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
							rawURLOrPath = u.JoinPath(rawURLOrPath).String()
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

						var buf *bytes.Buffer
						fetch := func(cred *credential.Credential) error {
							var err error
							buf, err = credential.Get(ctx, g, chartURL.String(), cred)
							return err
						}

						// The repository's credentials apply to charts on the
						// same host, as with Helm. Charts elsewhere only get
						// credentials scoped to their own host, so that an
						// index can't collect the repository's. Either way, a
						// rejection is joined into errs, so it isn't taken
						// for a rejection of the repository's credentials.
						if chartURL.Host == u.Host {
							err = fetch(cred)
						} else {
							err = credential.TryReferenced(ctx, chartURL, credential.Source, fetch)
						}
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

	buf, err := credential.Get(ctx, g, w.String(), cred)
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

	var buf *bytes.Buffer
	err = credential.Try(ctx, u, credential.Source, []string{scheme}, func(cred *credential.Credential) error {
		var err error
		buf, err = credential.Get(ctx, g, u.JoinPath("index.yaml").String(), cred)
		return err
	})
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
				t = u.JoinPath(rawURLOrPath)
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
