package barge_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/http"
	_ "github.com/frantjc/barge/internal/oci"
	_ "github.com/frantjc/barge/internal/repo"
	"github.com/frantjc/barge/testdata"
	"github.com/stretchr/testify/require"
)

func TestCopyArchive(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	directory := fmt.Sprintf("directory://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive.String(), directory))
}

func TestCopyDirectory(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	directory := fmt.Sprintf("directory://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive.String(), directory))
	require.NoError(t, barge.Copy(ctx, directory, t.TempDir()))
}

func TestCopyFile(t *testing.T) {
	ctx := Context(t)
	archiveChart, archive := Archive(t)
	file := fmt.Sprintf("file://%s/%s-%s.tgz", t.TempDir(), archiveChart.Name(), archiveChart.Metadata.Version)
	require.NoError(t, barge.Copy(ctx, archive.String(), file))
	require.NoError(t, barge.Copy(ctx, file, t.TempDir()))
	fileDir := fmt.Sprintf("file://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive.String(), fileDir))
	require.NoError(t, barge.Copy(ctx, fileDir, t.TempDir()))
}

func TestCopyDefault(t *testing.T) {
	ctx := Context(t)
	archiveChart, archive := Archive(t)
	file := fmt.Sprintf("%s/%s-%s.tgz", t.TempDir(), archiveChart.Name(), archiveChart.Metadata.Version)
	require.NoError(t, barge.Copy(ctx, archive.String(), file))
	require.NoError(t, barge.Copy(ctx, file, t.TempDir()))
	fileDir := t.TempDir()
	require.NoError(t, barge.Copy(ctx, archive.String(), fileDir))
	require.NoError(t, barge.Copy(ctx, fileDir, t.TempDir()))
}

func TestCopyHTTP(t *testing.T) {
	ctx := Context(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testdata.ChartArchive)
	}))
	t.Cleanup(srv.Close)
	archiveChart, _ := Archive(t)
	chartURL := fmt.Sprintf("%s/%s-%s.tgz", srv.URL, archiveChart.Name(), archiveChart.Metadata.Version)
	require.NoError(t, barge.Copy(ctx, chartURL, t.TempDir()))
}

func TestCopyRepo(t *testing.T) {
	ctx := Context(t)
	archiveChart, _ := Archive(t)
	repoName, _ := Repo(t, archiveChart)
	require.NoError(t, barge.Copy(ctx, fmt.Sprintf("repo://%s/%s?version=%s", repoName, archiveChart.Name(), archiveChart.Metadata.Version), t.TempDir()))
}

func TestCopyOCI(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	oci := OCI(t).JoinPath("test")
	require.NoError(t, barge.Copy(ctx, archive.String(), oci.String()))
	require.NoError(t, barge.Copy(ctx, oci.String(), t.TempDir()))
}

func TestCopyErrorUnknownScheme(t *testing.T) {
	ctx := Context(t)
	require.Error(t, barge.Copy(ctx, "foo://", t.TempDir()))
	require.Error(t, barge.Copy(ctx, fmt.Sprintf("file://%s", t.TempDir()), "bar://"))
}

func TestCopyErrorInvalid(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	ociURL := &url.URL{Scheme: "oci", Host: "does-not-exist"}
	require.Error(t, barge.Copy(ctx, archive.String(), ociURL.String()))
}
