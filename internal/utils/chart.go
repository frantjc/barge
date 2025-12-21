package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

// TODO(frantjc): Context propagation.

// WriteChartToDirectory writes the contents of the given
// chart to dir.
func WriteChartToDirectory(c *chart.Chart, dir string) error {
	// TODO(frantjc): Is it worth doing this in parallel?
	eg := new(errgroup.Group)

	if err := writeChart(c, func(data []byte, rel string) error {
		eg.Go(func() error {
			name := filepath.Join(dir, rel)

			if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
				return err
			}

			if err := os.WriteFile(name, data, 0644); err != nil {
				return err
			}

			return nil
		})

		return nil
	}); err != nil {
		return err
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// WriteChartToFile writes the gzipped tar contents of
// the given chart to the named file.
func WriteChartToFile(c *chart.Chart, name string) error {
	rc, err := WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = io.Copy(f, rc); err != nil {
		return err
	}

	return nil
}

// WriteChartToArchive returns a pipe that the gzipped tar
// contents of the given chart can be read from.
func WriteChartToArchive(c *chart.Chart) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		zw := gzip.NewWriter(pw)
		defer zw.Close()
		tw := tar.NewWriter(zw)

		if err := writeChart(c, func(data []byte, rel string) error {
			if err := tw.WriteHeader(&tar.Header{
				// Match `helm package`, which produces a tarball with
				// the name of the Chart as the base directory instead of ".".
				Name: path.Join(c.Name(), rel),
				Size: int64(len(data)),
			}); err != nil {
				return err
			}

			if _, err := tw.Write(data); err != nil {
				return err
			}

			return nil
		}); err != nil {
			pw.CloseWithError(err)
			return
		}

		pw.CloseWithError(zw.Close())
	}()

	return pr, nil
}

type marshall func(a any) ([]byte, error)

// writeChart is a modified version of helm.sh/helm/v3/pkg/chartutil.SaveDir.
func writeChart(c *chart.Chart, callback func(data []byte, rel string) error) error {
	var (
		marshallCallback = func(m marshall, a any, rel string) error {
			data, err := m(a)
			if err != nil {
				return err
			}
			return callback(data, rel)
		}
		yamlCallback = func(a any, rel string) error {
			return marshallCallback(yaml.Marshal, a, rel)
		}
	)

	// Pull out the dependencies of a v1 Chart, since there's no way
	// to tell the serializer to skip a field for just this use case.
	savedDependencies := c.Metadata.Dependencies
	if c.Metadata.APIVersion == chart.APIVersionV1 {
		c.Metadata.Dependencies = nil
	}
	defer func() {
		if c.Metadata.APIVersion == chart.APIVersionV1 {
			c.Metadata.Dependencies = savedDependencies
		}
	}()
	if err := yamlCallback(c.Metadata, chartutil.ChartfileName); err != nil {
		return err
	}

	if c.Metadata.APIVersion == chart.APIVersionV2 {
		if c.Lock != nil {
			if err := yamlCallback(c.Lock, "Chart.lock"); err != nil {
				return err
			}
		}
	}

	for _, f := range c.Raw {
		if f.Name == chartutil.ValuesfileName {
			if err := callback(f.Data, chartutil.ValuesfileName); err != nil {
				return err
			}
			break
		}
	}

	if c.Schema != nil {
		if err := marshallCallback(json.Marshal, c.Schema, chartutil.SchemafileName); err != nil {
			return err
		}
	}

	for _, f := range c.Templates {
		if err := callback(f.Data, f.Name); err != nil {
			return err
		}
	}

	for _, f := range c.Files {
		if err := callback(f.Data, f.Name); err != nil {
			return err
		}
	}

	for _, d := range c.Dependencies() {
		rc, err := WriteChartToArchive(c)
		if err != nil {
			return err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return err
		}

		if err := callback(data, path.Join(chartutil.ChartsDir, fmt.Sprintf("%s-%s.tgz", d.Name(), d.Metadata.Version))); err != nil {
			return err
		}
	}

	return nil
}
