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
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/assert"
)

var (
	oci string
)

func init() {
	flag.StringVar(&oci, "oci", "", "")
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
		if !strings.Contains(o, "://") {
			o = fmt.Sprintf("oci://%s", o)
		}
		u, err := url.Parse(o)
		assert.NoError(f, err)
		q := url.Values{}
		switch u.Scheme {
		case "oci", "":
		default:
			f.Fatalf("unknown scheme in -oci=%s", o)
		}
		oci := fmt.Sprintf("oci://%s?%s", path.Join(u.Host, u.Path), q.Encode())
		f.Add(archive, oci)
		f.Add(oci, archive)
	}

	f.Fuzz(func(t *testing.T, src, dest string) {
		assert.NoError(t, barge.Copy(t.Context(), src, dest))
	})
}
