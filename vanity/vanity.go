// © 2023 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

// Package vanity provides functionality for building a static site that lists
// Go packages from GitHub repositories. The package handles fetching repository
// data, generating documentation, and building the site using templates.
package vanity

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"go.astrophena.name/site"
)

// Logf is a simple printf-like logging function.
type Logf func(format string, args ...any)

// Write implements the [io.Writer] interface.
func (f Logf) Write(p []byte) (n int, err error) {
	f("%s", p)
	return len(p), nil
}

// Config holds the configuration for building the static site.
type Config struct {
	Dir         string       // Directory where the generated site will be stored.
	GitHubToken string       // GitHub token for accessing the GitHub API.
	ImportRoot  string       // Root import path for the Go packages.
	Logf        Logf         // Logger to use. If nil, log.Printf is used.
	HTTPClient  *http.Client // HTTP client for making requests.
}

type buildContext struct {
	c   *Config
	tpl *template.Template
}

//go:embed templates/*.html
var tplFS embed.FS

const highlightTheme = "native" // doc2go syntax highlighting theme

// Build constructs the static site by fetching repository data from GitHub,
// generating documentation, and building the site using templates.
func Build(ctx context.Context, c *Config) error {
	// Initialize internal state.
	if c.Logf == nil {
		c.Logf = Logf(log.Printf)
	}
	b := &buildContext{c: c}

	// Initialize templates.
	var err error
	b.tpl, err = template.New("vanity").Funcs(template.FuncMap{
		"contains":  strings.Contains,
		"hasOnePkg": b.hasOnePkg,
	}).ParseFS(tplFS, "templates/*.html")
	if err != nil {
		return err
	}

	// Obtain needed repositories from GitHub API.
	allRepos, err := doJSONRequest[[]*repo](ctx, c, http.MethodGet, "https://api.github.com/user/repos")
	if err != nil {
		return err
	}

	// Filter only Go modules.
	var repos []*repo
	for _, repo := range allRepos {
		if repo.Fork || repo.Name == "vanity" {
			continue
		}

		files, err := doJSONRequest[[]file](ctx, c, http.MethodGet, repo.URL+"/contents")
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.Path == "go.mod" {
				repos = append(repos, repo)
				break
			}
		}
	}

	// Clean up after previous build.
	if _, err := os.Stat(c.Dir); err == nil {
		if err := os.RemoveAll(c.Dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}

	// Create a temporary directory where generated site sources will be placed.
	tmpdir, err := os.MkdirTemp("", "vanity")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// Create subdirectories for various tasks.
	var (
		reposDir = filepath.Join(tmpdir, "repos")
		siteDir  = filepath.Join(tmpdir, "site")
	)
	for _, dir := range []string{reposDir, siteDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	// Compile the doc2go binary.
	c.Logf("Building doc2go.")
	doc2go := filepath.Join(tmpdir, "doc2go")
	install := exec.Command("go", "install", "go.abhg.dev/doc2go")
	install.Env = append(os.Environ(), "GOBIN="+filepath.Join(tmpdir))
	install.Stderr = c.Logf
	if err := install.Run(); err != nil {
		return err
	}

	for _, repo := range repos {
		// We don't list packages for private repos.
		if repo.Private {
			continue
		}

		if !strings.HasSuffix(repo.Description, ".") {
			repo.Description += "."
		}

		c.Logf("Cloning repository %s.", repo.Name)
		repo.Dir = filepath.Join(reposDir, repo.Name)
		clone := exec.Command("git", "clone", "--depth=1", repo.CloneURL, repo.Dir)
		clone.Stderr = c.Logf
		if err := clone.Run(); err != nil {
			return err
		}

		c.Logf("Running \"go list\" for %s.", repo.Name)
		var obuf, errbuf bytes.Buffer
		list := exec.Command("go", "list", "-json", "./...")
		list.Dir = repo.Dir
		list.Stdout = &obuf
		list.Stderr = &errbuf
		if err := list.Run(); err != nil {
			return fmt.Errorf("go list failed for repo %s: %v (it returned %q)", repo.Name, err, errbuf.String())
		}

		dec := json.NewDecoder(&obuf)
		for dec.More() {
			p := new(pkg)
			if err := dec.Decode(p); err != nil {
				return err
			}
			p.Repo = repo
			repo.Pkgs = append(repo.Pkgs, p)
		}
	}

	// Build repo and package pages.
	for _, repo := range repos {
		if repo.Dir != "" {
			c.Logf("Generating docs for %s.", repo.Name)
			git := exec.Command("git", "rev-parse", "--short", "HEAD")
			git.Dir = repo.Dir
			commitb, err := git.Output()
			if err != nil {
				return err
			}
			commitn := string(commitb)
			repo.Commit = strings.TrimSuffix(commitn, "\n")

			if err := repo.generateDoc(c, doc2go); err != nil {
				return err
			}
		}

		for _, pkg := range repo.Pkgs {
			pkg.BasePath = strings.TrimPrefix(pkg.ImportPath, c.ImportRoot+"/")
			if pkg.BasePath == repo.Name || strings.Contains(pkg.BasePath, "internal") {
				continue
			}

			if err := b.buildPage(filepath.Join(siteDir, "pages", pkg.BasePath+".html"), &site.Page{
				Title:     pkg.ImportPath,
				Template:  "main",
				Type:      "page",
				Permalink: "/" + pkg.BasePath,
				MetaTags:  metaTagsForRepo(c, repo),
			}, "pkg", pkg); err != nil {
				return err
			}
		}

		if err := b.buildPage(filepath.Join(siteDir, "pages", repo.Name+".html"), &site.Page{
			Title:       c.ImportRoot + "/" + repo.Name,
			Template:    "main",
			Type:        "page",
			Permalink:   "/" + repo.Name,
			MetaTags:    metaTagsForRepo(c, repo),
			ContentOnly: repo.Private,
		}, "import", repo); err != nil {
			return err
		}
	}

	// Build index page.
	if err := b.buildPage(filepath.Join(siteDir, "pages", "index.html"), &site.Page{
		Title:     "Go Packages",
		Template:  "main",
		Type:      "page",
		Permalink: "/",
	}, "index", repos); err != nil {
		return err
	}

	// Copy templates and static files from site.
	for _, dir := range []string{
		"pages/shared/",
		"static/css/",
		"static/icons/",
		"static/js/",
		"templates/",
	} {
		if err := copyDir(dir, filepath.Join(siteDir, dir)); err != nil {
			return err
		}
	}

	// Generate CSS for syntax highlighting.

	hcss, err := exec.Command(
		doc2go,
		"-highlight", highlightTheme,
		"-highlight-print-css",
	).Output()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(siteDir, "static"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(siteDir, "static", "css", "godoc.css"), hcss, 0o644); err != nil {
		return err
	}

	// Finally, build.
	return site.Build(&site.Config{
		Title: "Go Packages",
		BaseURL: &url.URL{
			Scheme: "https",
			Host:   "go.astrophena.name",
		},
		Src:      siteDir,
		Dst:      c.Dir,
		Prod:     true,
		SkipFeed: true,
		Vanity:   true,
	})
}

func (b *buildContext) buildPage(path string, page *site.Page, tmpl string, data any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	frontmatter, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Write(frontmatter)
	buf.WriteString("\n\n")

	if err := b.tpl.ExecuteTemplate(&buf, tmpl, data); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// copyDir copies a directory from source to destination, recursively.
// It sets permissions to 0o644 for files and 0o755 for directories.
func copyDir(source string, dest string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	if !sourceInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", source)
	}

	// Create destination dir with same permission bits.
	err = os.MkdirAll(dest, sourceInfo.Mode().Perm()|os.ModeDir)
	if err != nil {
		return err
	}

	files, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, file := range files {
		sourcePath := filepath.Join(source, file.Name())
		destPath := filepath.Join(dest, file.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := copyDir(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := copyFile(sourcePath, destPath, fileInfo.Mode().Perm()|0o400); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a file from source to destination with specific permissions.
func copyFile(source, dest string, perm os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if err := out.Chmod(perm); err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

type repo struct {
	// From GitHub API:
	Name        string `json:"name"`
	URL         string `json:"url"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	CloneURL    string `json:"clone_url"`
	Fork        bool   `json:"fork"`
	Owner       *owner `json:"owner"`
	// Obtained by 'git rev-parse --short HEAD'
	Commit string `json:"-"`
	// For use with doc2go
	Dir string `json:"-"`
	// Go packages that this repo contains
	Pkgs []*pkg `json:"-"`
}

type owner struct {
	Login string `json:"login"`
}

type pkg struct {
	// bits of 'go list -json' that we need.
	Name       string   // package name
	ImportPath string   // import path of package in dir
	Doc        string   // package documentation string
	GoFiles    []string // .go source files
	Imports    []string // import paths used by this package

	FullDoc string // generated by doc2go

	BasePath string

	Repo *repo
}

type file struct {
	Path string `json:"path"`
}

var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

func doJSONRequest[R any](ctx context.Context, c *Config, method, url string) (R, error) {
	var resp R

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.GitHubToken)

	httpc := defaultHTTPClient
	if c.HTTPClient != nil {
		httpc = c.HTTPClient
	}

	res, err := httpc.Do(req)
	if err != nil {
		return resp, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return resp, err
	}

	if res.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("%s %s: want %d, got %d: %s", method, url, http.StatusOK, res.StatusCode, b)
	}

	if err := json.Unmarshal(b, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

func (r *repo) generateDoc(c *Config, doc2goBin string) error {
	tmpdir, err := os.MkdirTemp("", "vanity-doc2go")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	doc2go := exec.Command(
		doc2goBin,
		"-highlight",
		"classes:"+highlightTheme,
		"-embed", "-out", tmpdir,
		"./...",
	)
	doc2go.Stderr = c.Logf
	doc2go.Dir = r.Dir
	if err := doc2go.Run(); err != nil {
		return err
	}

	for _, pkg := range r.Pkgs {
		docfile := filepath.Join(tmpdir, pkg.ImportPath, "index.html")
		if _, err := os.Stat(docfile); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}

		fullDoc, err := os.ReadFile(docfile)
		if err != nil {
			return err
		}
		pkg.FullDoc = string(fullDoc)
	}

	return nil
}

func metaTagsForRepo(c *Config, r *repo) map[string]string {
	return map[string]string{
		"go-import": fmt.Sprintf("%[1]s/%[2]s git https://github.com/%[3]s/%[2]s", c.ImportRoot, r.Name, r.Owner.Login),
	}
}

func (b *buildContext) hasOnePkg(r *repo) bool {
	if len(r.Pkgs) != 1 {
		return false
	}

	return r.Pkgs[0].ImportPath == b.c.ImportRoot+"/"+r.Name
}
