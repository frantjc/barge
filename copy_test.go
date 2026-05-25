package barge_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/frantjc/barge"
	_ "github.com/frantjc/barge/internal/archive"
	_ "github.com/frantjc/barge/internal/directory"
	_ "github.com/frantjc/barge/internal/file"
	_ "github.com/frantjc/barge/internal/oci"
	"github.com/stretchr/testify/require"
)

func TestCopyArchiveToDirectory(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	directory := fmt.Sprintf("directory://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive, directory))
}

func TestCopyDirectoryToDirectory(t *testing.T) {
	ctx := Context(t)

	_, archive := Archive(t)
	srcDir := fmt.Sprintf("directory://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive, srcDir))
	require.NoError(t, barge.Copy(ctx, srcDir, t.TempDir()))
}

func TestCopyArchiveToFile(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	fileDir := fmt.Sprintf("file://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive, fileDir))
}

func TestCopyFileToDirectory(t *testing.T) {
	ctx := Context(t)
	_, archive := Archive(t)
	fileDir := fmt.Sprintf("file://%s", t.TempDir())
	require.NoError(t, barge.Copy(ctx, archive, fileDir))
	require.NoError(t, barge.Copy(ctx, fileDir, t.TempDir()))
}

func TestCopyErrorUnknownScheme(t *testing.T) {
	require.Error(t, barge.Copy(t.Context(), "foo://", t.TempDir()))
	require.Error(t, barge.Copy(t.Context(), fmt.Sprintf("file://%s", t.TempDir()), "bar://"))
}

func TestCopyErrorInvalidOCI(t *testing.T) {
	ctx := Context(t)

	_, archive := Archive(t)
	ociURL := &url.URL{Scheme: "oci", Host: "does-not-exist"}
	require.Error(t, barge.Copy(ctx, archive, ociURL.String()))
}
