package barge_test

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/artifactory"
	_ "github.com/frantjc/barge/internal/chartmuseum"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/release"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/assert"
)

var (
	oci         string
	chartmuseum string
	repo        string
	http        string
)

func init() {
	flag.StringVar(&oci, "oci", "", "run copy tests against these comma-delimited registries")
	flag.StringVar(&chartmuseum, "cm", "", "run copy tests against these comma-delimited urls")
	flag.StringVar(&repo, "repo", "", "run copy tests against these comma-delimited repos")
	flag.StringVar(&http, "http", "", "run copy tests against these comma-delimited http(s)")
}

func FuzzCopy(f *testing.F) {
	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	assert.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	assert.NoError(f, err)
	assert.NoError(f, tmp.Close())
	archive := fmt.Sprintf("archive://%s", tmp.Name())
	directory := fmt.Sprintf("directory://%s", f.TempDir())
	file := f.TempDir()

	f.Add(archive, directory)
	f.Add(directory, archive)
	f.Add(archive, file)
	f.Add(file, archive)

	for _, o := range strings.Split(oci, ",") {
		if o == "" {
			continue
		} else if !strings.Contains(o, "://") {
			o = fmt.Sprintf("oci://%s", o)
		}
		u, err := url.Parse(o)
		assert.NoError(f, err)
		oci := fmt.Sprintf("oci://%s", path.Join(u.Host, u.Path))
		f.Add(archive, oci)
		f.Add(oci, archive)
	}

	for _, c := range strings.Split(chartmuseum, ",") {
		if c == "" {
			continue
		}
		u, err := url.Parse(c)
		assert.NoError(f, err)
		switch u.Scheme {
		case "cm", "chartmuseum":
		case "http":
			u.Scheme = "chartmuseum"
			q := u.Query()
			q.Set("insecure", "1")
			u.RawQuery = q.Encode()
		case "https":
			u.Scheme = "chartmuseum"
		default:
			f.Fatalf("unknown scheme in -cm=%s", c)
		}
		chartmuseum := u.String()
		f.Add(archive, chartmuseum)
	}

	for _, r := range strings.Split(repo, ",") {
		if r == "" {
			continue
		} else if !strings.Contains(r, "://") {
			r = fmt.Sprintf("repo://%s", r)
		}
		u, err := url.Parse(r)
		assert.NoError(f, err)
		repo := u.String()
		f.Add(repo, archive)
	}

	for _, h := range strings.Split(http, ",") {
		if h == "" {
			continue
		}
		u, err := url.Parse(h)
		assert.NoError(f, err)
		http := u.String()
		f.Add(http, archive)
	}

	f.Fuzz(func(t *testing.T, src, dest string) {
		assert.NoError(t, barge.Copy(t.Context(), src, dest))
	})
}

func FuzzCopyError(f *testing.F) {
	f.Add("foo://", f.TempDir())
	f.Add(f.TempDir(), "bar://")

	f.Fuzz(func(t *testing.T, src, dest string) {
		assert.Error(t, barge.Copy(t.Context(), src, dest))
	})
}
