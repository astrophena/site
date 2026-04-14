// © 2023 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

// Package vanity builds the go.astrophena.name site.
package vanity

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"text/template"

	"go.astrophena.name/base/logger"
	"go.astrophena.name/base/request"
	"go.astrophena.name/site/internal/site"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"
)

const pkgOverviewSelector = "h2#pkg-overview"

// Config configures a vanity-site build.
type Config struct {
	// Dir is the output directory for the generated site.
	Dir string
	// GitHubToken is used to access the GitHub API.
	GitHubToken string
	// ImportRoot is the root import path for the published packages.
	ImportRoot string
	// HTTPClient is used for GitHub API requests.
	HTTPClient *http.Client
	// RepoCacheDir, when set, stores persistent repository checkouts between builds.
	RepoCacheDir string
	// Concurrency limits how many repositories are prepared in parallel.
	Concurrency int
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
	if c == nil {
		return errors.New("nil config")
	}
	if c.Dir == "" || c.Dir == "." || c.Dir == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove unsafe build directory %q", c.Dir)
	}

	// Initialize internal state.
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

	// Fetch repositories visible to the authenticated user from GitHub.
	allRepos, err := listRepos(ctx, c)
	if err != nil {
		return err
	}

	// Keep only repositories whose root contains a go.mod file.
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

	// Remove any previous build output before generating the new site.
	if _, err := os.Stat(c.Dir); err == nil {
		if err := os.RemoveAll(c.Dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}

	// Create a temporary workspace for generated site sources.
	tmpdir, err := os.MkdirTemp("", "vanity")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	reposDir := filepath.Join(tmpdir, "repos")
	if c.RepoCacheDir != "" {
		reposDir = c.RepoCacheDir
	}
	siteDir := filepath.Join(tmpdir, "site")

	for _, dir := range []string{reposDir, siteDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(parallelism(c))
	for _, repo := range repos {
		repo := repo
		g.Go(func() error {
			return prepareRepo(gctx, c, repo, reposDir)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Build pages for packages and repository roots.
	for _, repo := range repos {
		// Public repositories with only internal packages still get a root page
		// that explains there are no importable packages.
		if repo.HasOnlyInternalPackages && !repo.Private {
			repo.Pkgs = []*pkg{
				{
					ImportPath: c.ImportRoot + "/" + repo.Name,
					BasePath:   repo.Name,
					Repo:       repo,
				},
			}
		}
		for _, pkg := range repo.Pkgs {
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

	// Ask doc2go for the syntax-highlighting stylesheet used by generated docs.

	hcss, err := exec.CommandContext(
		ctx,
		"go",
		"tool",
		"doc2go",
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

	// Build the final static site from the generated source tree.
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
	// Populated from the GitHub API.
	Name          string `json:"name"`
	URL           string `json:"url"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
	Archived      bool   `json:"archived"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Fork          bool   `json:"fork"`
	Owner         *owner `json:"owner"`
	// Commit is the short HEAD revision of the cloned repository.
	Commit string `json:"-"`
	// Dir is the local clone path used when generating docs.
	Dir string `json:"-"`
	// Pkgs contains the packages discovered in the repository.
	Pkgs []*pkg `json:"-"`
	// HasOnlyInternalPackages is true if all packages in Pkgs are internal.
	HasOnlyInternalPackages bool `json:"-"`
}

type owner struct {
	Login string `json:"login"`
}

type pkg struct {
	// Subset of fields returned by `go list -json`.
	Name       string   // package name
	ImportPath string   // import path of package in dir
	Doc        string   // package documentation string
	GoFiles    []string // .go source files
	Imports    []string // import paths used by this package

	// FullDoc is the HTML documentation generated by doc2go.
	FullDoc string

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

func listRepos(ctx context.Context, c *Config) ([]*repo, error) {
	var repos []*repo
	for page := 1; ; page++ {
		u := fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d", page)
		batch, err := makeRequest[[]*repo](ctx, c, u)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		repos = append(repos, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return repos, nil
}

func parallelism(c *Config) int {
	if c != nil && c.Concurrency > 0 {
		return c.Concurrency
	}
	if n := runtime.GOMAXPROCS(0); n > 0 {
		return n
	}
	return 1
}

func prepareRepo(ctx context.Context, c *Config, repo *repo, reposDir string) error {
	if repo.Private {
		// Private repositories get a synthetic root package page with access instructions.
		repo.Pkgs = []*pkg{{
			BasePath:   repo.Name,
			ImportPath: c.ImportRoot + "/" + repo.Name,
			Repo:       repo,
		}}
		return nil
	}

	if !strings.HasSuffix(repo.Description, ".") {
		repo.Description += "."
	}

	lg := logger.Get(ctx).With(slog.String("repo", repo.Name))
	repo.Dir = filepath.Join(reposDir, repo.Name)

	lg.Info("syncing checkout")
	if err := syncRepoCheckout(ctx, repo); err != nil {
		return err
	}

	commitb, err := runCommand(ctx, repo.Dir, nil, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}
	repo.Commit = strings.TrimSpace(string(commitb))

	lg.Info("running \"go list\"")
	pkgs, err := listRepoPackages(ctx, repo)
	if err != nil {
		return err
	}
	repo.Pkgs = pkgs
	repo.HasOnlyInternalPackages = hasOnlyInternalPackages(repo.Pkgs)
	if repo.HasOnlyInternalPackages {
		lg.Info("repository has only internal packages")
		repo.Pkgs = nil
		return nil
	}

	lg.Info("generating docs")
	return repo.generateDoc(ctx, c)
}

func syncRepoCheckout(ctx context.Context, repo *repo) error {
	gitDir := filepath.Join(repo.Dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		if err := updateRepoCheckout(ctx, repo); err == nil {
			return nil
		}
		if err := os.RemoveAll(repo.Dir); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if _, err := os.Stat(repo.Dir); err == nil {
		if err := os.RemoveAll(repo.Dir); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return cloneRepoCheckout(ctx, repo)
}

func cloneRepoCheckout(ctx context.Context, repo *repo) error {
	args := []string{"clone", "--depth=1"}
	if repo.DefaultBranch != "" {
		args = append(args, "--branch", repo.DefaultBranch, "--single-branch")
	}
	args = append(args, repo.CloneURL, repo.Dir)
	_, err := runCommand(ctx, "", nil, "git", args...)
	return err
}

func updateRepoCheckout(ctx context.Context, repo *repo) error {
	if _, err := runCommand(ctx, repo.Dir, nil, "git", "remote", "set-url", "origin", repo.CloneURL); err != nil {
		return err
	}

	if repo.DefaultBranch != "" {
		refspec := "refs/heads/" + repo.DefaultBranch
		if _, err := runCommand(ctx, repo.Dir, nil, "git", "fetch", "--depth=1", "origin", refspec); err != nil {
			return err
		}
		if _, err := runCommand(ctx, repo.Dir, nil, "git", "checkout", "--force", "-B", repo.DefaultBranch, "FETCH_HEAD"); err != nil {
			return err
		}
	} else {
		if _, err := runCommand(ctx, repo.Dir, nil, "git", "fetch", "--depth=1", "origin"); err != nil {
			return err
		}
		if _, err := runCommand(ctx, repo.Dir, nil, "git", "reset", "--hard", "FETCH_HEAD"); err != nil {
			return err
		}
	}
	_, err := runCommand(ctx, repo.Dir, nil, "git", "clean", "-fdx")
	return err
}

func listRepoPackages(ctx context.Context, repo *repo) ([]*pkg, error) {
	var (
		obuf   bytes.Buffer
		errbuf bytes.Buffer
		env    = append(os.Environ(), "GOTOOLCHAIN=auto")
	)
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	cmd.Env = env
	cmd.Dir = repo.Dir
	cmd.Stdout = &obuf
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list failed for repo %s: %v (it returned %q)", repo.Name, err, errbuf.String())
	}

	var pkgs []*pkg
	dec := json.NewDecoder(&obuf)
	for dec.More() {
		p := new(pkg)
		if err := dec.Decode(p); err != nil {
			return nil, err
		}
		p.Repo = repo
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %v (it returned %q)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type file struct {
	Path string `json:"path"`
}

func (r *repo) generateDoc(ctx context.Context, c *Config) error {
	tmpdir, err := os.MkdirTemp("", "vanity-doc2go")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	rootImportPath := path.Join(c.ImportRoot, r.Name)
	haveRootPkg := slices.ContainsFunc(r.Pkgs, func(pkg *pkg) bool {
		return pkg.ImportPath == rootImportPath
	})
	if !haveRootPkg {
		r.Pkgs = append(r.Pkgs, &pkg{
			ImportPath: rootImportPath,
			Repo:       r,
		})
	}

	doc2go := exec.CommandContext(
		ctx,
		"go",
		"tool",
		"doc2go",
		"-C", r.Dir,
		"-highlight",
		"classes:"+highlightTheme,
		"-pkg-doc", rootImportPath+"=https://{{ .ImportPath }}",
		"-embed", "-out", tmpdir,
		"./...",
	)
	if err := doc2go.Run(); err != nil {
		return err
	}

	for _, pkg := range r.Pkgs {
		pkg.BasePath = strings.TrimPrefix(pkg.ImportPath, c.ImportRoot+"/")

		docfile := filepath.Join(tmpdir, pkg.ImportPath, "index.html")
		if _, err := os.Stat(docfile); errors.Is(err, fs.ErrNotExist) {
			continue
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

func isRelativePackageLink(link string) bool {
	if link == "" || strings.HasPrefix(link, "#") {
		return false
	}
	if strings.HasPrefix(link, "/") || isFullURL(link) {
		return false
	}
	if strings.HasPrefix(link, "//") {
		return false
	}
	if strings.Contains(link, ":") {
		return false
	}
	return true
}

func isInternalImportPath(importPath string) bool {
	if importPath == "internal" || strings.HasPrefix(importPath, "internal/") {
		return true
	}
	return strings.HasSuffix(importPath, "/internal") || strings.Contains(importPath, "/internal/")
}

func hasOnlyInternalPackages(pkgs []*pkg) bool {
	if len(pkgs) == 0 {
		return false
	}
	return !slices.ContainsFunc(pkgs, func(pkg *pkg) bool {
		return !isInternalImportPath(pkg.ImportPath)
	})
}

func (p *pkg) modifyHTML(c *Config) error {
	p.replaceRelLinks(c)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(p.FullDoc))
	if err != nil {
		return err
	}

	overview := doc.Find(pkgOverviewSelector).First()
	if overview.Length() > 0 {
		var sections []string
		if p.Name == "main" {
			sections = append(sections, installSnippet(p.ImportPath))
		}
		if toc := buildTOCSnippet(doc); toc != "" {
			sections = append(sections, toc)
		}
		if len(sections) > 0 {
			overview.AfterHtml(strings.Join(sections, ""))
		}
	}

	html, err := doc.Html()
	if err != nil {
		return err
	}
	p.FullDoc = html

	return nil
}

func buildTOCSnippet(doc *goquery.Document) string {
	var (
		headings int
		toc      strings.Builder
	)

	toc.WriteString("<h3>Contents</h3><ul>\n")
	doc.Find("[id^=hdr-]").Each(func(_ int, s *goquery.Selection) {
		id, exists := s.Attr("id")
		if !exists {
			return
		}
		fmt.Fprintf(&toc, "<li><a href=\"#%s\">%s</a></li>\n",
			html.EscapeString(id),
			html.EscapeString(s.Text()))
		headings++
	})
	toc.WriteString("</ul>\n")

	if headings <= 1 {
		return ""
	}
	return toc.String()
}

func installSnippet(importPath string) string {
	return fmt.Sprintf(
		"<p>Install this program:</p><pre><code>$ go install %s@latest</code></pre>",
		html.EscapeString(importPath),
	)
}

var (
	hrefRe     = regexp.MustCompile(`href="(.*?)"`)
	fragmentRe = regexp.MustCompile(`^(.*?)(#(.*))?$`)
)

func (p *pkg) replaceRelLinks(c *Config) {
	// Rewrite doc2go's module-relative links so they point at vanity-hosted pages.
	basePath := "/" + strings.TrimPrefix(p.ImportPath, c.ImportRoot+"/")

	replaceLink := func(link string) string {
		if strings.HasPrefix(link, "../") {
			absPath := path.Join(basePath, link)
			absPath = path.Clean(absPath)
			return absPath
		}
		if isRelativePackageLink(link) {
			absPath := path.Join(basePath, link)
			return absPath
		}
		if isFullURL(link) || strings.HasPrefix(link, "#") || strings.Contains(link, ":") || strings.HasPrefix(link, "//") {
			return link
		}
		return path.Clean(link)
	}

	p.FullDoc = hrefRe.ReplaceAllStringFunc(p.FullDoc, func(match string) string {
		parts := strings.Split(match, `"`)
		link := parts[1]

		newLink := replaceLink(link)

		if path, frag := linkFragment(newLink); frag != "" && !isFullURL(newLink) {
			newLink = pathpkgClean(path) + "#" + frag
		}

		return fmt.Sprintf(`href="%s"`, newLink)
	})
}

func pathpkgClean(p string) string {
	if p == "" {
		return ""
	}
	return path.Clean(p)
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
