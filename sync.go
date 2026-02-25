package barge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/frantjc/barge/internal/util"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"
)

type URL url.URL

func (j *URL) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	u, err := url.Parse(raw)
	if err != nil {
		return err
	}

	*j = URL(*u)
	return nil
}

func (j URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j URL) String() string {
	u := url.URL(j)
	return u.String()
}

func (j URL) JoinPath(elem ...string) URL {
	u := url.URL(j)
	return URL(*u.JoinPath(elem...))
}

func (j URL) Query() url.Values {
	u := url.URL(j)
	return u.Query()
}

type Constraints semver.Constraints

func (j *Constraints) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c, err := semver.NewConstraint(raw)
	if err != nil {
		return err
	}

	*j = Constraints(*c)
	return nil
}

func (j Constraints) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j Constraints) String() string {
	return semver.Constraints(j).String()
}

func (j Constraints) Check(version *Version) bool {
	v := semver.Version(*version)
	return semver.Constraints(j).Check(&v)
}

type SourceConfig struct {
	URL       URL                    `json:"url"`
	Namespace string                 `json:"namespace,omitempty"`
	Charts    map[string]Constraints `json:"charts,omitempty"`
}

type SyncConfig struct {
	Sources []SourceConfig `json:"sources"`
}

type SyncOpts struct {
	FailFast bool
}

type SyncOpt interface {
	Apply(*SyncOpts)
}

func (s *SyncOpts) Apply(opts *SyncOpts) {
	if s != nil {
		if opts != nil {
			opts.FailFast = s.FailFast
		}
	}
}

func (s *SyncConfig) Sync(ctx context.Context, dest string, opts ...SyncOpts) error {
	o := &SyncOpts{}

	for _, opt := range opts {
		opt.Apply(o)
	}

	log := util.SloggerFrom(ctx)

	d, err := url.Parse(os.ExpandEnv(dest))
	if err != nil {
		return err
	}

	if d.Scheme == "" {
		d.Scheme = "file"
	}

	destination, ok := destMux[d.Scheme]
	if !ok {
		return fmt.Errorf("no destination registered for scheme: %s", d.Scheme)
	}

	syncableDestination, ok := destination.(SyncableDestination)
	if !ok {
		return fmt.Errorf("not a syncable destination scheme: %s", d.Scheme)
	}

	eg := new(errgroup.Group)
	if o.FailFast {
		eg, ctx = errgroup.WithContext(ctx)
	}

	for _, src := range s.Sources {
		eg.Go(func() error {
			namespace := src.Namespace

			s, err := url.Parse(os.ExpandEnv(src.URL.String()))
			if err != nil {
				return err
			}

			if s.Scheme == "" {
				s.Scheme = "file"
			}

			source, ok := srcMux[s.Scheme]
			if !ok {
				return fmt.Errorf("no source registered for scheme: %s", s.Scheme)
			}

			if len(src.Charts) > 0 {
				syncableSource, ok := source.(SyncableSource)
				if !ok {
					return fmt.Errorf("not a queryable source scheme: %s", s.Scheme)
				}

				for name, constraints := range src.Charts {
					syncableVersions, err := syncableSource.Versions(ctx, s, name)
					if err != nil {
						return err
					}

					for _, syncableVersion := range syncableVersions {
						if constraints.Check(syncableVersion.Version) {
							eg.Go(func() error {
								syncSource := source

								if syncableVersion.URL.Scheme != s.Scheme {
									syncSource, ok = srcMux[syncableVersion.URL.Scheme]
									if !ok {
										return fmt.Errorf("no source registered for scheme: %s", syncableVersion.URL.Scheme)
									}
								}

								chart, err := syncSource.Open(ctx, (*url.URL)(syncableVersion.URL))
								if err != nil {
									return err
								}

								log.Info("syncing", "chart", chart.Name(), "version", chart.Metadata.Version, "destination", d.String(), "namespace", namespace)

								return syncableDestination.Sync(ctx, d, namespace, chart)
							})
						}
					}
				}

				return nil
			}

			chart, err := source.Open(ctx, s)
			if err != nil {
				return err
			}

			return syncableDestination.Sync(ctx, d, namespace, chart)
		})
	}

	return eg.Wait()
}

func Sync(ctx context.Context, cfg, dest string) error {
	if cfg == "-" {
		cfg = "/dev/stdin"
	}

	b, err := os.ReadFile(cfg)
	if err != nil {
		return err
	}

	s := &SyncConfig{}
	if err := yaml.Unmarshal(b, s); err != nil {
		return err
	}

	return s.Sync(ctx, dest)
}
