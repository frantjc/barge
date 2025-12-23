// A generated module for Sindri functions

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/frantjc/barge/.dagger/internal/dagger"
	xslices "github.com/frantjc/x/slices"
)

type BargeDev struct {
	Source *dagger.Directory
}

func New(
	ctx context.Context,
	// +optional
	// +defaultPath="."
	src *dagger.Directory,
) (*BargeDev, error) {
	return &BargeDev{
		Source: src,
	}, nil
}

func (m *BargeDev) Fmt(ctx context.Context) *dagger.Changeset {
	goModules := []string{
		".dagger/",
	}

	root := dag.Go(dagger.GoOpts{
		Module: m.Source.Filter(dagger.DirectoryFilterOpts{
			Exclude: goModules,
		}),
	}).
		Container().
		WithExec([]string{"go", "fmt", "./..."}).
		Directory(".")

	for _, module := range goModules {
		root = root.WithDirectory(
			module,
			dag.Go(dagger.GoOpts{
				Module: m.Source.Directory(module).Filter(dagger.DirectoryFilterOpts{
					Exclude: xslices.Filter(goModules, func(m string, _ int) bool {
						return strings.HasPrefix(m, module)
					}),
				}),
			}).
				Container().
				WithExec([]string{"go", "fmt", "./..."}).
				Directory("."),
		)
	}

	return root.Changes(m.Source)
}

func (m *BargeDev) Test(
	ctx context.Context,
	// +optional
	oci []string,
) (*dagger.Container, error) {
	chartmuseum := dag.Container().
		From("ghcr.io/helm/chartmuseum:v0.16.3").
		WithExposedPort(8080).
		WithEnvVariable("DEBUG", "1").
		WithEnvVariable("STORAGE", "local").
		WithEnvVariable("STORAGE_LOCAL_ROOTDIR", "/tmp").
		AsService()
	chartmuseumAlias := "chartmuseum"
	chartmuseumURL := fmt.Sprintf("http://%s:8080", chartmuseumAlias)
	chartmuseumRepo := chartmuseumAlias

	registry := dag.Container().
		From("docker.io/distribution/distribution:3").
		WithExposedPort(5000).
		AsService()
	registryAlias := "registry"

	test := []string{
		"go", "test", "-race", "-cover", "-test.v",
		"-cm",
		chartmuseumURL,
		"-oci",
		strings.Join([]string{
			fmt.Sprintf("%s:5000/test", registryAlias),
			fmt.Sprintf("%s:5000/test:tag", registryAlias),
		}, ","),
		"-repo",
		strings.Join([]string{
			fmt.Sprintf("%s/test", chartmuseumRepo),
			fmt.Sprintf("%s/test/0.2.0", chartmuseumRepo),
		}, ","),
		"-http",
		fmt.Sprintf("%s/charts/test-0.2.0.tgz", chartmuseumURL),
	}
	test = append(test, "./...")

	return dag.Go(dagger.GoOpts{
		Module:                  m.Source,
		AdditionalWolfiPackages: []string{"helm-4", "curl"},
	}).
		Container().
		WithServiceBinding(chartmuseumAlias, chartmuseum).
		WithExec([]string{"curl", "-X", "POST", "-F", "chart=@testdata/test-0.2.0.tgz", fmt.Sprintf("%s/api/charts", chartmuseumURL)}).
		WithExec([]string{"helm", "repo", "add", chartmuseumRepo, chartmuseumURL}).
		WithServiceBinding(registryAlias, registry).
		WithExec(test), nil
}

func (m *BargeDev) Release(ctx context.Context, githubToken *dagger.Secret) error {
	gitRef := m.Source.AsGit().LatestVersion()

	ref, err := gitRef.Ref(ctx)
	if err != nil {
		return err
	}

	tag := strings.TrimPrefix(ref, "refs/tags/")

	release := dag.Wolfi().
		Container(dagger.WolfiContainerOpts{
			Packages: []string{"gh"},
		}).
		WithSecretVariable("GITHUB_TOKEN", githubToken).
		WithExec([]string{"gh", "release", "-R=frantjc/barge", "create", tag, "--generate-notes", "--draft"})

	g0 := dag.Go(dagger.GoOpts{
		Module: gitRef.Tree(),
	})

	for _, goos := range []string{"darwin", "linux"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			file := fmt.Sprintf("barge_%s_%s_%s", tag, goos, goarch)

			release = release.
				WithFile(
					file,
					g0.Build(dagger.GoBuildOpts{
						Pkg:     "./cmd/barge",
						Ldflags: "-s -w -X main.version=" + tag,
						Goos:    goos,
						Goarch:  goarch,
					}),
				).
				WithExec([]string{
					"gh", "release", "-R=frantjc/barge", "upload", tag, file,
				})
		}
	}

	_, err = release.
		WithExec([]string{"gh", "release", "-R=frantjc/barge", "edit", tag, "--latest", "--draft=false"}).
		Sync(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (m *BargeDev) Binary(
	ctx context.Context,
	// +default=v0.0.0-unknown
	version string,
	// +optional
	goarch string,
	// +optional
	goos string,
) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/barge",
			Ldflags: "-s -w -X main.version=" + version,
			Goos:    goos,
			Goarch:  goarch,
		})
}

func (m *BargeDev) Vulncheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "install", "golang.org/x/vuln/cmd/govulncheck@v1.1.4"}).
		WithExec([]string{"govulncheck", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Vet(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "vet", "./..."}).
		CombinedOutput(ctx)
}

func (m *BargeDev) Staticcheck(ctx context.Context) (string, error) {
	return dag.Go(dagger.GoOpts{
		Module: m.Source,
	}).
		Container().
		WithExec([]string{"go", "install", "honnef.co/go/tools/cmd/staticcheck@v0.6.1"}).
		WithExec([]string{"staticcheck", "./..."}).
		CombinedOutput(ctx)
}
