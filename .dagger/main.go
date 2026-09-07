package main

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/frantjc/barge/.dagger/internal/dagger"
)

type BargeDev struct{}

// +check
func (m *BargeDev) Test(
	ctx context.Context,
	ws *dagger.Workspace,
	// +optional
	githubToken *dagger.Secret,
	// +optional
	githubRepo,
	// +optional
	acrName string,
	// +optional
	azureConfig *dagger.Directory,
) error {
	cluster := dag.Kwok().Cluster()
	alias := "kwok"
	tags := []string{"dagger", "examples", "kubernetes"}
	tools := []string{"go", "helm"}
	if azureConfig != nil && acrName != "" {
		tools = append(tools, "azure-cli")
	}
	return dag.Go(dagger.GoOpts{
		Ws: ws,
		Container: dag.Mise(dagger.MiseOpts{
			Ws: ws,
		}).
			Container(dagger.MiseContainerOpts{
				Tools: tools,
			}).
			With(func(r *dagger.Container) *dagger.Container {
				if azureConfig != nil && acrName != "" {
					tags = append(tags, "acr")
					return r.
						WithEnvVariable("ACR_NAME", acrName).
						WithMountedDirectory("$HOME/.azure", azureConfig, dagger.ContainerWithMountedDirectoryOpts{
							Expand: true,
						})
				}
				return r
			}).
			With(func(r *dagger.Container) *dagger.Container {
				if githubToken != nil && githubRepo != "" {
					tags = append(tags, "ghcr", "github")
					return r.
						WithSecretVariable("GITHUB_TOKEN", githubToken).
						WithEnvVariable("GITHUB_REPOSITORY", githubRepo)
				}
				return r
			}).
			WithServiceBinding(alias, cluster.Container().AsService()).
			WithEnvVariable("KUBECONFIG", "$HOME/.kube/config", dagger.ContainerWithEnvVariableOpts{
				Expand: true,
			}).
			WithFile("$KUBECONFIG", cluster.KubeConfig(dagger.KwokClusterKubeConfigOpts{Alias: alias}), dagger.ContainerWithFileOpts{
				Expand: true,
			}),
	}).
		Test(ctx, dagger.GoTestOpts{
			Race: true,
			Tags: tags,
		})
}

func (m *BargeDev) Binary(
	ctx context.Context,
	ws *dagger.Workspace,
	// +default=v0.0.0-unknown
	version,
	// +optional
	goarch,
	// +optional
	goos string,
) *dagger.File {
	return dag.Go(dagger.GoOpts{
		Ws: ws,
	}).
		Build(dagger.GoBuildOpts{
			Pkg:     "./cmd/barge",
			Ldflags: "-s -w -X main.version=" + version,
			Goos:    goos,
			Goarch:  goarch,
		})
}

func (m *BargeDev) Release(
	ctx context.Context,
	ws *dagger.Workspace,
	githubToken *dagger.Secret,
	// +optional
	githubRepo string,
	// +optional
	brew bool,
) error {
	_, repo, ok := strings.Cut(githubRepo, "/")
	if !ok {
		return fmt.Errorf("expected org/repo format, got %q", githubRepo)
	}

	gh := dag.Gh(githubToken)
	src := ws.Directory(".", dagger.WorkspaceDirectoryOpts{
		Gitignore: true,
	})

	gitRepository := src.AsGit()
	latestVersion := gitRepository.LatestVersion()

	ref, err := latestVersion.Ref(ctx)
	if err != nil {
		return err
	}
	tag := strings.TrimPrefix(ref, "refs/tags/")

	assets := []*dagger.File{}

	for _, goos := range []string{"linux", "darwin"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			bin := m.Binary(ctx, ws, tag, goarch, goos)

			if goos == "linux" {
				bin = dag.Upx().Pack(bin)
			}

			file := fmt.Sprintf("%s-%s-%s-%s.tar.gz", repo, tag, goos, goarch)
			asset := dag.Archive().
				Tar(
					src.Filter(dagger.DirectoryFilterOpts{
						Include: []string{
							"README.md",
							"LICENSE",
						},
					}).
						WithFile(
							repo,
							bin,
						),
					dagger.ArchiveTarOpts{
						Gzip: true,
					},
				).WithName(file)

			assets = append(assets, asset)
		}
	}

	release := gh.Release(githubRepo, tag)

	if err := release.Create(ctx, dagger.GhReleaseCreateOpts{
		Draft:         true,
		GenerateNotes: true,
	}); err != nil {
		return err
	}

	if err := release.Upload(ctx, assets, dagger.GhReleaseUploadOpts{
		Clobber: true,
	}); err != nil {
		return err
	}

	if brew {
		if err := dag.Homebrew().Cask(ctx, githubToken, githubRepo, tag); err != nil {
			return err
		}
	}

	if err := release.Edit(ctx, dagger.GhReleaseEditOpts{
		Latest: true,
	}); err != nil {
		return err
	}

	return nil
}
