// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

/*
Package site builds https://astrophena.name.

# Directory Structure

Site has the following directories:

	build      This is where the generated site will be placed by default.
	pages      All content for the site lives inside this directory. HTML and
	           Markdown formats can be used.
	static     Files in this directory will be copied verbatim to the
	           generated site.
	templates  These are the templates that wrap pages. Templates are
	           chosen on a page-by-page basis in the front matter.
	           They must have the '.html' extension.

# Page Layout

Each page must be of the supported format (HTML or Markdown) and have JSON front
matter in the beginning:

	{
	  "title": "Hello, world!",
	  "template": "layout",
	  "permalink": "/hello-world"
	}

See Page for all available front matter fields.
*/
package site

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	ttemplate "text/template"
	"time"

	"go.astrophena.name/base/logger"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/feeds"
	"rsc.io/markdown"
)

// Possible errors, used in tests.
var (
	errFrontmatterSplit        = errors.New("failed to split frontmatter and contents")
	errFrontmatterParse        = errors.New("failed to parse frontmatter")
	errFrontmatterMissing      = errors.New("missing frontmatter")
	errFrontmatterMissingParam = errors.New("missing required frontmatter parameter (title, template, permalink)")
	errFormatUnsupported       = errors.New("format unsupported")
	errPermalinkInvalid        = errors.New("invalid permalink")
)

// Config represents a build configuration.
type Config struct {
	// Title is the title of the site.
	Title string
	// Author is the name of the author of the site.
	Author string
	// BaseURL is the base URL of the site.
	BaseURL *url.URL
	// Src is the directory where to read files from. If empty, uses the current
	// directory.
	Src string
	// Dst is the directory where to write files. If empty, uses the build
	// directory.
	Dst string
	// Logf specifies a logger to use. If nil, log.Printf is used.
	Logf logger.Logf
	// Prod determines if the site should be built in a production mode. This
	// means that drafts are excluded and the base URL is used to derive absolute
	// URLs from relative ones.
	Prod bool
	// SkipFeed determines if the feed for site shouldn't be built.
	SkipFeed bool
	// Vanity determines if the site is vanity import domain built with vanity
	// package. If so, navigation links created with navLink will point to URLs
	// derived from PrimaryURL instead of BaseURL.
	Vanity bool
	// PrimaryURL is the base URL for navigation links when Vanity set to true.
	PrimaryURL *url.URL

	feedCreated time.Time // used in tests
}

func (c *Config) setDefaults() {
	if c == nil {
		c = &Config{}
	}

	if c.Logf == nil {
		c.Logf = log.Printf
	}

	if c.Title == "" {
		c.Title = "Ilya Mateyko"
	}

	if c.Author == "" {
		c.Author = "Ilya Mateyko"
	}

	if c.BaseURL == nil {
		c.BaseURL = &url.URL{
			Scheme: "https",
			Host:   "astrophena.name",
		}
	}
	if c.PrimaryURL == nil {
		c.PrimaryURL = &url.URL{
			Scheme: "https",
			Host:   "astrophena.name",
		}
	}

	if c.Src == "" {
		c.Src = filepath.Join(".")
	}

	if c.Dst == "" {
		c.Dst = filepath.Join(".", "build")
	}
}

// Build builds a site based on the provided [Config].
func Build(c *Config) error {
	c.setDefaults()
	b := newBuildContext(c)

	// Parse templates and pages.
	if err := filepath.WalkDir(filepath.Join(b.c.Src, "templates"), b.parseTemplates); err != nil {
		return err
	}
	if err := filepath.WalkDir(filepath.Join(b.c.Src, "pages"), b.parsePages); err != nil {
		return err
	}

	// Sort pages by date. Pages without date are pushed to the end.
	sort.SliceStable(b.pages, func(i, j int) bool {
		if b.pages[i].Date == nil || b.pages[j].Date == nil {
			return true
		}
		return !b.pages[i].Date.Time.Before(b.pages[j].Date.Time)
	})

	// Clean up after previous build.
	if _, err := os.Stat(b.c.Dst); err == nil {
		if err := os.RemoveAll(b.c.Dst); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(b.c.Dst, 0o755); err != nil {
		return err
	}

	// Build pages and RSS feed.
	for _, p := range b.pages {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(b.c.Dst, p.dstPath)), 0o755); err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(b.c.Dst, p.dstPath))
		if err != nil {
			return err
		}
		defer f.Close()

		tpl, ok := b.templates[p.Template]
		if !ok {
			return fmt.Errorf("%s: no such template %q", p.path, p.Template)
		}
		if err := p.build(b, tpl, f); err != nil {
			return err
		}
	}
	if !b.c.SkipFeed {
		if err := b.buildFeed(); err != nil {
			return err
		}
	}

	// Copy static files.
	if err := os.CopyFS(b.c.Dst, os.DirFS(filepath.Join(b.c.Src, "static"))); err != nil {
		return err
	}

	return nil
}

var serveReadyHook func() // used in tests, called when Serve started serving the site

// Serve builds the site and starts serving it on a provided host:port.
func Serve(ctx context.Context, c *Config, addr string) error {
	c.setDefaults()

	c.Logf("Performing an initial build...")
	if err := Build(c); err != nil {
		c.Logf("Initial build failed: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	for _, dir := range []string{
		filepath.Join(c.Src, "pages"),
		filepath.Join(c.Src, "static"),
		filepath.Join(c.Src, "templates"),
	} {
		if err := watchRecursive(watcher, dir); err != nil {
			return err
		}
	}
	defer watcher.Close()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	c.Logf("Listening on http://%s...", l.Addr().String())

	httpSrv := &http.Server{Handler: &staticHandler{fs: os.DirFS(c.Dst)}}
	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(l); err != nil {
			if err != http.ErrServerClosed {
				errCh <- err
			}
		}
	}()

	go func() {
		c.Logf("Started watching for new changes.")

		// FIXME: Debounce closer changes.

		var (
			changes   = make(chan struct{}, 1) // buffered to avoid blocking
			buildDone = make(chan struct{})    // signals when a build is done
		)

		go func() {
			defer close(changes)

			var pending bool

			for {
				select {
				case event := <-watcher.Events:
					if !shouldRebuild(event.Name, event.Op) {
						continue
					}

					if pending {
						c.Logf("Detected change %s (%v), but build is in progress.", event.Name, event.Op)
						continue
					}

					c.Logf("Detected change %s (%v), triggering build.", event.Name, event.Op)
					pending = true
					changes <- struct{}{}
				case <-buildDone:
					if pending {
						c.Logf("Accumulated changes detected, triggering new build.")
						changes <- struct{}{}
						pending = false
						continue
					}

					c.Logf("Build completed with no further changes.")
					pending = false
				case <-ctx.Done():
					return
				}
			}
		}()

		go func() {
			defer close(buildDone)
			for {
				select {
				case <-changes:
					if err := Build(c); err != nil {
						c.Logf("Failed to rebuild the site: %v", err)
					}
					buildDone <- struct{}{}
				case <-ctx.Done():
					return
				}
			}
		}()
	}()

	if serveReadyHook != nil {
		serveReadyHook()
	}

	select {
	case <-ctx.Done():
		c.Logf("Gracefully shutting down...")
	case <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return httpSrv.Shutdown(shutdownCtx)
}

func watchRecursive(w *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.Add(path)
	})
}

// Copied from
// https://github.com/brandur/modulir/blob/1ff912fdc45a79cb4d8d9f199d213ae9c3598cbd/watch.go#L201.
func shouldRebuild(path string, op fsnotify.Op) bool {
	base := filepath.Base(path)

	// Mac OS' worst mistake.
	if base == ".DS_Store" {
		return false
	}

	// Vim creates this temporary file to see whether it can write into a target
	// directory. It screws up our watching algorithm, so ignore it.
	if base == "4913" {
		return false
	}

	// A special case, but ignore creates on files that look like Vim backups.
	if strings.HasSuffix(base, "~") {
		return false
	}

	if op&fsnotify.Create != 0 {
		return true
	}

	if op&fsnotify.Remove != 0 {
		return true
	}

	if op&fsnotify.Write != 0 {
		return true
	}

	/*
		Ignore everything else. Rationale:

		* chmod: we don't really care about these as they won't affect build
		output (unless potentially we no longer can read the file, but we'll go
		down that path if it ever becomes a problem).

		* rename: will produce a following create event as well, so just listen
		for that instead.
	*/
	return false
}

type staticHandler struct {
	fs fs.FS
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/" {
		p += "/index.html"
	}
	p = strings.TrimPrefix(path.Clean(p), "/")

	// Special case: /foo will serve content from foo.html, if it exists.
	if _, err := fs.Stat(h.fs, p+".html"); err == nil {
		p += ".html"
	}

	d, err := fs.Stat(h.fs, p)
	if errors.Is(err, fs.ErrNotExist) {
		h.serveNotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if d.IsDir() {
		h.serveNotFound(w, r)
		return
	}

	b, err := fs.ReadFile(h.fs, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, d.Name(), d.ModTime(), bytes.NewReader(b))
}

func (h *staticHandler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	f, err := h.fs.Open("404.html")
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.WriteHeader(http.StatusNotFound)
	io.Copy(w, f)
}

type buildContext struct {
	c         *Config
	md        *markdown.Parser
	funcs     template.FuncMap
	pages     []*Page
	templates map[string]*template.Template
}

func newBuildContext(c *Config) *buildContext {
	b := &buildContext{
		c: c,
		md: &markdown.Parser{
			HeadingID:          true,
			Strikethrough:      true,
			TaskList:           true,
			AutoLinkText:       true,
			AutoLinkAssumeHTTP: true,
			Table:              true,
			Emoji:              true,
			SmartDot:           true,
			SmartDash:          true,
			SmartQuote:         true,
			Footnote:           true,
		},
		templates: make(map[string]*template.Template),
	}

	b.funcs = template.FuncMap{
		"content":   func(p *Page) template.HTML { return template.HTML(p.contents) },
		"time":      b.time,
		"icon":      b.icon,
		"image":     b.image,
		"navLink":   b.navLink,
		"pages":     b.pagesByType,
		"url":       b.url,
		"vanity":    func() bool { return b.c.Vanity },
		"vanityURL": b.vanityURL,
	}

	return b
}

func (b *buildContext) icon(name string) template.HTML {
	return template.HTML(fmt.Sprintf(`
<svg class="icon" aria-hidden="true">
  <use xlink:href="%s#icon-%s"/>
</svg>`, b.url("/icons/sprite.svg"), name))
}

func (b *buildContext) image(path, caption string) template.HTML {
	const tmpl = `<figure>
  <img alt="%[2]s" src="%[1]s" loading="lazy"/>
  <figcaption>%[2]s</figcaption>
</figure>`
	s := fmt.Sprintf(tmpl, b.url(path), caption)
	return template.HTML(s)
}

func (b *buildContext) navLink(p *Page, title, iconName, path string) template.HTML {
	var highlight bool

	// On vanity site always highlight packages link, and nothing else.
	if b.c.Vanity {
		if path == b.c.BaseURL.String() {
			highlight = true
		}
	} else if p.Permalink == path {
		highlight = true
	}

	var add string
	if highlight {
		add = ` class="current"`
	}
	var u string
	if b.c.Vanity && b.c.PrimaryURL != nil {
		u = b.vanityURL(path)
	} else {
		u = b.url(path)
	}
	return template.HTML(fmt.Sprintf(`<a href="%s"%s>%s%s</a>`, u, add, b.icon(iconName), title))
}

func (b *buildContext) pagesByType(typ string) []*Page {
	if typ == "" {
		return b.pages
	}
	var pages []*Page
	for _, p := range b.pages {
		if p.Type == typ {
			pages = append(pages, p)
		}
	}
	return pages
}

func (b *buildContext) time(format string, d *date) template.HTML {
	return template.HTML(fmt.Sprintf(`<date datetime="%s">%s</date>`,
		d.Format(time.RFC3339),
		d.Format(format),
	))
}

func isFullURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (b *buildContext) url(base string) string {
	if isFullURL(base) || !b.c.Prod || b.c.BaseURL == nil {
		return base
	}
	u := *b.c.BaseURL
	u.Path = path.Join(u.Path, base)
	return u.String()
}

func (b *buildContext) vanityURL(base string) string {
	if isFullURL(base) {
		return base
	}
	u := *b.c.PrimaryURL
	u.Path = path.Join(u.Path, base)
	return u.String()
}

func (b *buildContext) parseTemplates(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

	if filepath.Ext(path) != ".html" {
		return nil
	}

	name, err := filepath.Rel(filepath.Join(b.c.Src, "templates"), path)
	if err != nil {
		return err
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	// Ensure that we have slash-separated path everywhere.
	name = filepath.ToSlash(name)

	bb, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b.templates[name], err = template.New(name).Funcs(b.funcs).Parse(string(bb))
	if err != nil {
		return err
	}

	return nil
}

func (b *buildContext) parsePages(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

	// Ignore files that look like Vim backups.
	if strings.HasSuffix(path, "~") {
		return nil
	}

	// Ignore .gitignore files.
	if strings.Contains(path, ".gitignore") {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	p := &Page{path: path}
	if err := p.parse(f); err != nil {
		return err
	}
	if !p.Draft || !b.c.Prod {
		b.pages = append(b.pages, p)
	}

	return nil
}

// Page represents a site page. The exported fields is the front matter fields.
type Page struct {
	Title       string            `json:"title"`                  // title: Page title, required.
	Permalink   string            `json:"permalink"`              // permalink: Output path for the page, required.
	Template    string            `json:"template"`               // template: Template that should be used for rendering this page, required.
	ContentOnly bool              `json:"content_only,omitempty"` // content_only: Determines whether this page should be rendered without header and footer, false by default.
	Date        *date             `json:"date,omitempty"`         // date: Publication date in the 'year-month-day' format, e.g. 2006-01-02, optional.
	Draft       bool              `json:"draft,omitempty"`        // draft: Determines whether this page should be not included in production builds, false by default.
	MetaTags    map[string]string `json:"meta_tags,omitempty"`    // meta_tags: Determines additional HTML meta tags that will be added to this page, optional.
	Summary     string            `json:"summary,omitempty"`      // summary: Page summary, used in RSS feed, optional.
	Type        string            `json:"type,omitempty"`         // type: Used to distinguish different kinds of pages, page by default.
	CSS         []string          `json:"css,omitempty"`          // css: Additional CSS files that should be loaded, optional.
	JS          []string          `json:"js,omitempty"`           // js: Additional JavaScript files that should be loaded, optional.

	path     string // path to the page source
	dstPath  string // where to write the built page
	contents []byte // page contents without front matter
}

type date struct {
	time.Time
}

const dateLayout = "2006-01-02"

func (d *date) UnmarshalJSON(p []byte) error {
	s := strings.Trim(string(p), "\"")
	if s == "null" {
		d.Time = time.Time{}
		return nil
	}

	dt, err := time.Parse(dateLayout, s)
	if err != nil {
		return err
	}
	d.Time = dt

	return nil
}

func (p *Page) parse(r io.Reader) error {
	// Check that format of the page is supported.
	var supported bool
	for _, f := range []string{".html", ".md"} {
		if filepath.Ext(p.path) == f {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("%s: %w", p.path, errFormatUnsupported)
	}

	const (
		leftDelim  = "{\n"
		rightDelim = "}\n"
	)

	// Split the front matter and contents.
	scanner := bufio.NewScanner(r)
	var (
		frontmatter, contents []byte
		reachedFrontmatter    bool
		reachedContents       bool
	)
	for scanner.Scan() {
		line := scanner.Text() + "\n"

		if !reachedContents {
			if line == leftDelim {
				reachedFrontmatter = true
			}

			if line == rightDelim {
				reachedFrontmatter = false
				frontmatter = append(frontmatter, line...)
				reachedContents = true
				continue
			}
		}

		if reachedFrontmatter {
			frontmatter = append(frontmatter, line...)
			continue
		}

		if reachedContents {
			contents = append(contents, line...)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w: %v", p.path, errFrontmatterSplit, err)
	}
	if len(frontmatter) == 0 {
		return fmt.Errorf("%s: %w", p.path, errFrontmatterMissing)
	}
	p.contents = contents

	// Parse the front matter.
	if err := json.Unmarshal(frontmatter, p); err != nil {
		return fmt.Errorf("%s: %w: %v", p.path, errFrontmatterParse, err)
	}
	// Set the default page type.
	if p.Type == "" {
		p.Type = "page"
	}

	// Check front matter fields.
	if p.Title == "" || p.Template == "" || p.Permalink == "" {
		return fmt.Errorf("%s: %w", p.path, errFrontmatterMissingParam)
	}
	if _, err := url.ParseRequestURI(p.Permalink); err != nil {
		return fmt.Errorf("%s: %w: %v", p.path, errPermalinkInvalid, err)
	}
	p.dstPath = p.Permalink
	if !strings.HasSuffix(p.dstPath, ".html") {
		if p.dstPath == "/" {
			p.dstPath = p.dstPath + "index"
		}
		p.dstPath = p.dstPath + ".html"
	}
	p.dstPath = path.Clean(p.dstPath)

	return nil
}

var htmlCommentRe = regexp.MustCompile("<!--(.*?)-->")

func (p *Page) build(b *buildContext, tpl *template.Template, w io.Writer) error {
	// We use here text/template, but not html/template because we don't want to
	// escape any HTML on the Markdown source.
	ptpl, err := ttemplate.New(p.path).Funcs(ttemplate.FuncMap(b.funcs)).Parse(string(p.contents))
	if err != nil {
		return err
	}
	var pbuf bytes.Buffer
	if err = ptpl.Execute(&pbuf, p); err != nil {
		return fmt.Errorf("%s: failed to execute page template: %w", p.path, err)
	}
	p.contents = pbuf.Bytes()

	if filepath.Ext(p.path) == ".md" {
		doc := b.md.Parse(string(p.contents))
		p.contents = []byte(markdown.ToHTML(doc))
	}

	p.contents = htmlCommentRe.ReplaceAll(p.contents, []byte{})

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, p); err != nil {
		return fmt.Errorf("%s: failed to execute template %q: %w", p.path, p.Template, err)
	}

	_, err = buf.WriteTo(w)
	return err
}

func (b *buildContext) buildFeed() error {
	feed := &feeds.Feed{
		Title:   b.c.Title,
		Link:    &feeds.Link{Href: b.c.BaseURL.String() + "/"},
		Author:  &feeds.Author{Name: b.c.Author},
		Created: time.Now(),
	}

	if !b.c.feedCreated.IsZero() {
		feed.Created = b.c.feedCreated
	}

	for _, p := range b.pages {
		if p.Type != "post" {
			continue
		}

		if p.Draft && b.c.Prod {
			continue
		}

		pu := *b.c.BaseURL
		pu.Path = path.Join(pu.Path, p.Permalink)

		item := &feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: pu.String()},
			Author:      feed.Author,
			Description: p.Summary,
			Content:     string(p.contents),
		}
		if p.Date != nil {
			item.Created = p.Date.Time
		}
		feed.Items = append(feed.Items, item)
	}

	bf, err := feed.ToAtom()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.c.Dst, "feed.xml"), []byte(bf), 0o644)
}
