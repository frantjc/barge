package barge_test

import (
	"fmt"
	"net/url"
	"os"
	"path"
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
	rawOci = os.Getenv("TEST_BARGE_OCI")
)

func FuzzCopy(f *testing.F) {
	tmp, err := os.CreateTemp(f.TempDir(), "test-0.1.0.tgz")
	assert.NoError(f, err)
	_, err = tmp.Write(testdata.ChartArchive)
	assert.NoError(f, err)
	assert.NoError(f, tmp.Close())
	archive := fmt.Sprintf("archive://%s", tmp.Name())
	directory := fmt.Sprintf("directory://%s", f.TempDir())

	f.Add(archive, directory)
	f.Add(directory, archive)

	if rawOci != "" {
		u, err := url.Parse(rawOci)
		assert.NoError(f, err)
		q := url.Values{}
		switch u.Scheme {
		case "http":
			q.Add("insecure", "1")
		case "https", "oci", "":
		default:
			f.Fatalf("unknown scheme for TEST_BARGE_OCI: %s", rawOci)
		}
		oci := fmt.Sprintf("oci://%s?%s", path.Join(u.Host, u.Path), q.Encode())
		f.Add(archive, oci)
		// f.Add(oci, archive)
	}

	f.Fuzz(func(t *testing.T, src, dest string) {
		assert.NoError(t, barge.Copy(t.Context(), src, dest))
	})
}
