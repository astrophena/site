// © 2023 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

// Package vanity builds https://go.astrophena.name.
package vanity

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"go.astrophena.name/base/logger"
	"go.astrophena.name/base/request"
	"go.astrophena.name/site"

	"github.com/PuerkitoBio/goquery"
)

// Config represents a build configuration.
type Config struct {
	// Dir is a directory where the generated site will be stored.
	Dir string
	// GitHubToken is a token for accessing the GitHub API.
	GitHubToken string
	// ImportRoot is a root import path for the Go packages.
	ImportRoot string
	// Logf is a logger to use. If nil, log.Printf is used.
	Logf logger.Logf
	// HTTPClient is a HTTP client for making requests.
	HTTPClient *http.Client
}

type buildContext struct {
	c   *Config
	tpl *template.Template
}

//go:embed templates/*.html
var tplFS embed.FS

const highlightTheme = "native" // doc2go syntax highlighting theme

// Build builds a site based on the provided [Config].
func Build(ctx context.Context, c *Config) error {
	// Initialize internal state.
	if c.Logf == nil {
		c.Logf = logger.Logf(log.Printf)
	}
	b := &buildContext{c: c}

	// Initialize templates.
	var err error
	b.tpl, err = template.New("vanity").Funcs(template.FuncMap{
		"contains":   strings.Contains,
		"hasOnePkg":  b.hasOnePkg,
		"importRoot": func() string { return c.ImportRoot },
	}).ParseFS(tplFS, "templates/*.html")
	if err != nil {
		return err
	}

	// Obtain needed repositories from GitHub API.
	allRepos, err := makeRequest[[]*repo](ctx, c, "https://api.github.com/user/repos")
	if err != nil {
		return err
	}

	// Filter only Go modules.
	var repos []*repo
	for _, repo := range allRepos {
		if repo.Fork || repo.Name == "vanity" {
			continue
		}

		files, err := makeRequest[[]file](ctx, c, repo.URL+"/contents")
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
		if repo.Private {
			// For private repos, we create a single virtual package.
			repo.Pkgs = []*pkg{
				&pkg{
					BasePath:   repo.Name,
					ImportPath: c.ImportRoot + "/" + repo.Name,
					Repo:       repo,
				},
			}
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
			if strings.Contains(pkg.BasePath, "internal") {
				continue
			}

			if err := b.buildPage(filepath.Join(siteDir, "pages", pkg.BasePath+".html"), &site.Page{
				Title:       pkg.ImportPath,
				Template:    "main",
				Type:        "page",
				Permalink:   "/" + pkg.BasePath,
				MetaTags:    metaTagsForRepo(c, repo),
				ContentOnly: repo.Private,
			}, "pkg", pkg); err != nil {
				return err
			}
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
		if err := os.CopyFS(filepath.Join(siteDir, dir), os.DirFS(dir)); err != nil {
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
			Host:   c.ImportRoot,
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

func makeRequest[Response any](ctx context.Context, c *Config, url string) (Response, error) {
	return request.Make[Response](ctx, request.Params{
		Method: http.MethodGet,
		URL:    url,
		Headers: map[string]string{
			"Authorization": "Bearer " + c.GitHubToken,
		},
		HTTPClient: c.HTTPClient,
	})
}

type file struct {
	Path string `json:"path"`
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
		"-pkg-doc", path.Join(c.ImportRoot, r.Name)+"=https://{{ .ImportPath }}",
		"-embed", "-out", tmpdir,
		"./...",
	)
	doc2go.Stderr = c.Logf
	doc2go.Dir = r.Dir
	if err := doc2go.Run(); err != nil {
		return err
	}

	// If we don't have a package which import path equals the module path
	// (e.g. for github.com/astrophena/go-testrepo module there's no package
	// "github.com/astrophena/go-testrepo", only subpackages like
	// "github.com/astrophena/go-testrepo/http"), then we create such a
	// package manually.
	haveRootPkg := false
	for _, pkg := range r.Pkgs {
		if pkg.ImportPath == c.ImportRoot+"/"+r.Name {
			haveRootPkg = true
			break
		}
	}
	if !haveRootPkg {
		r.Pkgs = append(r.Pkgs, &pkg{
			ImportPath: c.ImportRoot + "/" + r.Name,
			Repo:       r,
		})
	}

	for _, pkg := range r.Pkgs {
		pkg.BasePath = strings.TrimPrefix(pkg.ImportPath, c.ImportRoot+"/")

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
		if err := pkg.modifyHTML(c); err != nil {
			return err
		}
	}

	return nil
}

func isFullURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

func (p *pkg) modifyHTML(c *Config) error {
	p.replaceRelLinks(c)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(p.FullDoc))
	if err != nil {
		return err
	}

	if p.Name == "main" {
		doc.Find("h2#pkg-overview").AfterHtml(fmt.Sprintf("<pre id\"command\"##code>$ go install %s</code></pre>", p.ImportPath))
	}

	var (
		needTOC bool
		toc     strings.Builder
	)
	toc.WriteString("<h2>Table of Contents</h2><ul>\n")

	doc.Find("[id^=hdr-]").Each(func(i int, s *goquery.Selection) {
		id, exists := s.Attr("id")
		if !exists {
			return
		}
		text := s.Text()
		toc.WriteString(fmt.Sprintf("<li><a href=\"#%s\">%s</a></li>\n", id, text))
		needTOC = true
	})
	toc.WriteString("</ul>\n")

	tocTarget := "h2#pkg-overview"
	if p.Name == "main" {
		tocTarget = "pre#command"
	}
	if needTOC {
		doc.Find(tocTarget).AfterHtml(toc.String())
	}

	html, err := doc.Html()
	if err != nil {
		return err
	}
	p.FullDoc = html

	return nil
}

var (
	hrefRe     = regexp.MustCompile(`href="(.*?)"`)
	fragmentRe = regexp.MustCompile(`^(.*?)(#(.*))?$`)
)

func (p *pkg) replaceRelLinks(c *Config) {
	// Calculate the correct base path for relative links.
	// For example, if the package is "go.astrophena.name/base/testutil",
	// the base path will be "/base/testutil".
	basePath := "/" + strings.TrimPrefix(p.ImportPath, c.ImportRoot+"/")

	// Define a function to handle link replacements.
	replaceLink := func(link string) string {
		// If the link starts with "../", it's a relative link within the module.
		if strings.HasPrefix(link, "../") {
			// Calculate the absolute path by navigating up the directory structure.
			absPath := filepath.Join(basePath, link)
			// Clean the path to remove any unnecessary "./" or "../" segments.
			absPath = filepath.Clean(absPath)
			return absPath
		}
		// If the link doesn't contain a slash, it's a relative link to the package
		// root. The same case for missing slash in the beginning.
		if !strings.Contains(link, "/") || (!strings.HasPrefix(link, "/") && !isFullURL(link)) {
			absPath := filepath.Join(basePath, link)
			return absPath
		}
		// If it's not a relative link within the module, return it cleaned, at
		// least.
		if isFullURL(link) {
			return link
		}
		return filepath.Clean(link)
	}

	// Use a regular expression to find all links in the documentation.
	p.FullDoc = hrefRe.ReplaceAllStringFunc(p.FullDoc, func(match string) string {
		// Extract the actual link from the matched string.
		parts := strings.Split(match, `"`)
		link := parts[1]

		// Replace the link if necessary.
		newLink := replaceLink(link)

		// Handle links with fragments.
		if path, frag := linkFragment(newLink); frag != "" && !isFullURL(newLink) {
			newLink = filepath.Clean(path) + "#" + frag
		}

		// Return the modified match with the updated link.
		return fmt.Sprintf(`href="%s"`, newLink)
	})
}

func linkFragment(link string) (path string, fragment string) {
	matches := fragmentRe.FindStringSubmatch(link)
	if len(matches) == 4 {
		return matches[1], matches[3]
	}
	return link, ""
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
