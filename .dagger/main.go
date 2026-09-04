package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
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

var (
	//go:embed cask.rb.tpl
	caskRbTpl string
)

type tplOsArchData struct {
	URL    string
	Sha256 string
}
type tplData struct {
	Name        string
	Homepage    string
	Description string
	Version     string
	OsArch      map[string]map[string]tplOsArchData
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
	tpl, err := template.New("cask").Parse(caskRbTpl)
	if err != nil {
		return err
	}
	data := new(tplData)
	owner, repo, ok := strings.Cut(githubRepo, "/")
	if !ok {
		return fmt.Errorf("expected org/repo format, got %q", githubRepo)
	}
	data.Name = repo
	data.Homepage = fmt.Sprintf("https://github.com/%s", githubRepo)

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
	data.Version = strings.TrimPrefix(ref, "refs/tags/")

	description, err := gh.Container().
		WithExec([]string{"gh", "repo", "view", githubRepo, "--json", "description", "--jq", ".description"}).
		Stdout(ctx)
	if err != nil {
		return err
	}
	data.Description = strings.TrimSpace(description)

	assets := []*dagger.File{}

	version, err := dag.Version(ctx)
	if err != nil {
		return err
	}

	for _, goos := range []string{"linux", "darwin"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			bin := m.Binary(ctx, ws, version, goarch, goos)

			if goos == "linux" {
				bin = dag.Upx().Pack(bin)
			}

			file := fmt.Sprintf("%s-%s-%s-%s.tar.gz", data.Name, data.Version, goos, goarch)
			asset := dag.Archive().
				Tar(
					src.Filter(dagger.DirectoryFilterOpts{
						Include: []string{
							"README.md",
							"LICENSE",
						},
					}).
						WithFile(
							data.Name,
							bin,
						),
					dagger.ArchiveTarOpts{
						Gzip: true,
					},
				).WithName(file)

			sha256sum, err := dag.Wolfi().
				Container().
				WithFile(file, asset).
				WithExec([]string{"sha256sum", file}).
				Stdout(ctx)
			if err != nil {
				return err
			}

			checksum, _, _ := strings.Cut(sha256sum, "  ")

			if data.OsArch == nil {
				data.OsArch = map[string]map[string]tplOsArchData{}
			}
			osArchData := tplOsArchData{
				URL:    fmt.Sprintf("%s/releases/download/%s/%s", data.Homepage, data.Version, file),
				Sha256: strings.TrimPrefix(checksum, "sha256:"),
			}
			os := "linux"
			if goos == "darwin" {
				os = "macos"
			}
			arch := "intel"
			if goarch == "arm64" {
				arch = "arm"
			}
			if _, ok := data.OsArch[goos]; ok {
				data.OsArch[os][arch] = osArchData
			} else {
				data.OsArch[os] = map[string]tplOsArchData{arch: osArchData}
			}

			assets = append(assets, asset)
		}
	}

	release := gh.Release(githubRepo, data.Version)

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
		buf := new(bytes.Buffer)
		enc := base64.NewEncoder(base64.StdEncoding, buf)

		if err := tpl.Execute(enc, data); err != nil {
			return err
		}

		if err = enc.Close(); err != nil {
			return err
		}

		endpoint := fmt.Sprintf("repos/%s/homebrew-tap/contents/Casks/%s.rb", owner, data.Name)
		upload := []string{
			"gh",
			"api",
			"-X=PUT",
			endpoint,
			"-f", fmt.Sprintf("message=chore: bump %s to %s", data.Name, data.Version),
			"-f", fmt.Sprintf("content=%s", buf.String()),
		}

		if sha, err := gh.Container().
			WithExec([]string{
				"gh",
				"api",
				endpoint,
				"--jq",
				".sha",
			}).
			Stdout(ctx); err == nil {
			upload = append(upload, "-f", fmt.Sprintf("sha=%s", strings.TrimSpace(sha)))
		}

		if _, err := gh.Container().
			WithExec(upload).
			Sync(ctx); err != nil {
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
