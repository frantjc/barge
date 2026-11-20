package barge_test

import (
	"testing"

	"github.com/frantjc/barge"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// archiveWithVersion writes the test chart with the given version to an
// archive and returns its URL.
func archiveWithVersion(t *testing.T, version string) string {
	t.Helper()
	c, _ := Archive(t)
	c.Metadata.Version = version
	path, err := chartutil.Save(c, t.TempDir())
	require.NoError(t, err)
	return "archive://" + path
}

// pulledVersion copies src to a directory and returns the copied chart's
// version.
func pulledVersion(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, barge.Copy(Context(t), src, dir))
	c, err := loader.LoadDir(dir)
	require.NoError(t, err)
	return c.Metadata.Version
}

func TestCopyOCITagsChartVersion(t *testing.T) {
	t.Parallel()
	ctx := Context(t)
	c, archive := Archive(t)
	oci := OCI(t).JoinPath("test")

	require.NoError(t, barge.Copy(ctx, archive.String(), oci.String()))

	require.NoError(t, barge.Copy(ctx, oci.String()+"?version="+c.Metadata.Version, t.TempDir()))
	require.Error(t, barge.Copy(ctx, oci.String()+":latest", t.TempDir()))
}

func TestCopyOCIExplicitTag(t *testing.T) {
	t.Parallel()
	ctx := Context(t)
	_, archive := Archive(t)
	oci := OCI(t).JoinPath("test")

	require.NoError(t, barge.Copy(ctx, archive.String(), oci.String()+":custom"))
	require.NoError(t, barge.Copy(ctx, oci.String()+":custom", t.TempDir()))
}

func TestCopyOCIPullsHighestSemverTag(t *testing.T) {
	t.Parallel()
	oci := OCI(t).JoinPath("test").String()

	for _, version := range []string{"0.2.0", "0.10.0", "0.9.0", "0.11.0-rc.1"} {
		require.NoError(t, barge.Copy(Context(t), archiveWithVersion(t, version), oci))
	}

	// Highest by semver, not lexically, and a prerelease is lower than its
	// release but higher than prior releases.
	require.Equal(t, "0.11.0-rc.1", pulledVersion(t, oci))
	require.Equal(t, "0.9.0", pulledVersion(t, oci+"?version=0.9.0"))
}

func TestCopyOCIBuildMetadataVersion(t *testing.T) {
	t.Parallel()
	oci := OCI(t).JoinPath("test").String()

	// OCI tags can't contain "+", so Helm tags this as 0.1.0_build.1.
	require.NoError(t, barge.Copy(Context(t), archiveWithVersion(t, "0.1.0+build.1"), oci))
	require.Equal(t, "0.1.0+build.1", pulledVersion(t, oci))
}

// Charts that barge pushed before it tagged them with their version are still
// pulled from :latest.
func TestCopyOCIFallsBackToLatest(t *testing.T) {
	t.Parallel()
	ctx := Context(t)
	_, archive := Archive(t)
	oci := OCI(t).JoinPath("test").String()

	require.NoError(t, barge.Copy(ctx, archive.String(), oci+":latest"))
	require.NoError(t, barge.Copy(ctx, oci, t.TempDir()))
}
