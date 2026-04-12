package git

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/util"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	github "github.com/google/go-github/v72/github"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func init() {
	barge.RegisterSource(
		new(source),
		"git",
		"git+file",
		"git+http",
		"git+https",
		"git+ssh",
	)
}

type source struct{}

func (s *source) Open(ctx context.Context, u *url.URL) (*chart.Chart, error) {
	remote, rel := remoteAndPathFromURL(ctx, u)
	ref := u.Query().Get("ref")
	_ = util.SloggerFrom(ctx)

	if remote.Hostname() == "github.com" {
		return s.openGitHub(ctx, remote, rel, ref)
	}

	return s.openGit(ctx, remote, rel, ref)
}

func (s *source) openGitHub(ctx context.Context, remote *url.URL, rel, ref string) (*chart.Chart, error) {
	owner, repo, err := ownerAndRepo(remote)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(githubHTTPClient(remote))
	archiveURL, _, err := client.Repositories.GetArchiveLink(ctx, owner, repo, github.Tarball, &github.RepositoryContentGetOptions{Ref: ref}, 3)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := githubHTTPClient(remote).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("get github tarball: %s", res.Status)
	}

	return loadChartFromTarball(res.Body, rel)
}

func (s *source) openGit(ctx context.Context, remote *url.URL, rel, ref string) (*chart.Chart, error) {
	fs, err := clone(ctx, remote, ref)
	if err != nil {
		return nil, err
	}

	return loadChartFromFilesystem(fs, rel)
}

func clone(ctx context.Context, remote *url.URL, ref string) (billy.Filesystem, error) {
	auth := authMethod(remote)
	options := []*git.CloneOptions{{
		URL:   remote.String(),
		Depth: 1,
		Tags:  git.NoTags,
		Auth:  auth,
	}}

	if ref != "" {
		options = append([]*git.CloneOptions{
			{
				URL:           remote.String(),
				Depth:         1,
				SingleBranch:  true,
				ReferenceName: plumbing.NewBranchReferenceName(ref),
				Tags:          git.NoTags,
				Auth:          auth,
			},
			{
				URL:           remote.String(),
				Depth:         1,
				SingleBranch:  true,
				ReferenceName: plumbing.NewTagReferenceName(ref),
				Tags:          git.NoTags,
				Auth:          auth,
			},
		}, options...)

		if strings.HasPrefix(ref, "refs/") {
			options = append([]*git.CloneOptions{{
				URL:           remote.String(),
				Depth:         1,
				SingleBranch:  true,
				ReferenceName: plumbing.ReferenceName(ref),
				Tags:          git.NoTags,
				Auth:          auth,
			}}, options...)
		}
	}

	var err error
	for _, opt := range options {
		fs := memfs.New()
		_, err = git.CloneContext(ctx, memory.NewStorage(), fs, opt)
		if err == nil {
			return fs, nil
		}
	}

	return nil, err
}

func authMethod(remote *url.URL) *githttp.BasicAuth {
	if remote == nil || remote.User == nil {
		return nil
	}

	password, ok := remote.User.Password()
	if !ok || password == "" {
		return nil
	}

	return &githttp.BasicAuth{
		Username: remote.User.Username(),
		Password: password,
	}
}

func loadChartFromTarball(r io.Reader, rel string) (*chart.Chart, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	// Find the root directory in the tarball (GitHub adds a prefix)
	var rootPrefix string
	files := []*loader.BufferedFile{}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Skip pax headers and directories
		if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		// Detect root prefix from first regular file
		if rootPrefix == "" {
			parts := strings.Split(header.Name, "/")
			if len(parts) > 0 {
				rootPrefix = parts[0] + "/"
			}
		}

		// Remove root prefix and check if it's in our target subdirectory
		name := strings.TrimPrefix(header.Name, rootPrefix)

		// If rel is specified, only include files under that path
		if rel != "" && rel != "." {
			relPath := rel
			if !strings.HasSuffix(relPath, "/") {
				relPath = relPath + "/"
			}
			if !strings.HasPrefix(name, relPath) {
				continue
			}
			name = strings.TrimPrefix(name, relPath)
		}

		// Skip if name is empty after trimming
		if name == "" {
			continue
		}

		// Read file data
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		files = append(files, &loader.BufferedFile{
			Name: name,
			Data: data,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in subdirectory %q", rel)
	}

	return loader.LoadFiles(files)
}

func loadChartFromFilesystem(fs billy.Filesystem, rel string) (*chart.Chart, error) {
	root := "/"
	if cleaned := cleanRelativePath(rel); cleaned != "" {
		root = path.Join("/", cleaned)
	}

	files := []*loader.BufferedFile{}
	if err := walkFiles(fs, root, "", &files); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("chart path %q not found in repository", rel)
	}

	return loader.LoadFiles(files)
}

func walkFiles(fs billy.Filesystem, root, prefix string, files *[]*loader.BufferedFile) error {
	entries, err := fs.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		full := path.Join(root, name)
		rel := name
		if prefix != "" {
			rel = path.Join(prefix, name)
		}

		if entry.IsDir() {
			if err := walkFiles(fs, full, rel, files); err != nil {
				return err
			}
			continue
		}

		f, err := fs.Open(full)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}

		*files = append(*files, &loader.BufferedFile{Name: rel, Data: data})
	}

	return nil
}

func cleanRelativePath(rel string) string {
	if rel == "" || rel == "." || rel == "/" {
		return ""
	}

	return strings.TrimPrefix(path.Clean("/"+rel), "/")
}

func ownerAndRepo(remote *url.URL) (string, string, error) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(remote.Path, "/"), "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid github repository path: %s", remote.Path)
	}

	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func githubHTTPClient(remote *url.URL) *http.Client {
	if remote == nil || remote.User == nil {
		return http.DefaultClient
	}

	password, ok := remote.User.Password()
	if !ok || password == "" {
		return http.DefaultClient
	}

	username := remote.User.Username()
	transport := http.DefaultTransport

	return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.SetBasicAuth(username, password)
		return transport.RoundTrip(clone)
	})}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func remoteAndPathFromURL(ctx context.Context, u *url.URL) (*url.URL, string) {
	repoPath := u.Path
	rel := "."

	// Check for explicit // separator first
	if before, after, ok := strings.Cut(u.Path, "//"); ok {
		repoPath = before
		if trimmed := strings.TrimPrefix(after, "/"); trimmed != "" {
			rel = trimmed
		}
	} else if u.Hostname() == "github.com" {
		// For GitHub URLs without //, parse as owner/repo/subpath
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 2 {
			// github.com/owner/repo/subpath... -> owner/repo is repo, rest is subpath
			repoPath = "/" + strings.Join(parts[:2], "/")
			rel = strings.Join(parts[2:], "/")
		}
	}

	remote := *u
	remote.RawQuery = ""
	remote.Fragment = ""
	remote.Path = repoPath

	if scheme, ok := strings.CutPrefix(remote.Scheme, "git+"); ok {
		remote.Scheme = scheme
	}

	if remote.User == nil {
		if username, password, ok := util.UsernameAndPasswordForURLWithEnvFallback(u, util.LocationSource, "git"); ok {
			remote.User = url.UserPassword(username, password)
		} else if remote.Hostname() == "github.com" {
			if username, password, err := util.GetGitHubAuth(ctx); err == nil && password != "" {
				remote.User = url.UserPassword(username, password)
			}
		}
	}

	return &remote, rel
}
