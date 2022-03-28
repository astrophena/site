// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

// Package site builds https://astrophena.name.
//
// In most cases it is used from the build.go tool:
//
//  $ ./build.go
//
// To see all available flags, run:
//
//  $ ./build.go -help
//
// Directory Structure
//
// Site has the following directories:
//
//  build      This is where the generated site will be placed by default.
//  pages      All content for the site lives inside this directory.
//  static     Files in this directory will be copied verbatim to the
//             generated site.
//  templates  These are the templates that wrap pages. Templates are
//             chosen on a page-by-page basis in the front matter.
//             They must have the '.html' extension.
//
// Page Layout
//
// Each page must be of the supported format (see SupportedFormats) and have
// JSON front matter in the beginning:
//
//  {
//    "title": "Hello, world!",
//    "template": "layout",
//    "permalink": "/hello-world"
//  }
//
// See Page for all available front matter fields.
//
// Template Functions
//
// In templates, the following functions can be used:
//
//  {{ content page }}            Returns the page content.
//
//  {{ formatDate format date }}  Formats the given date based on the supplied
//                                format.
//
//  {{ env }}                     Returns an environment used for
//                                building the site.
//
//  {{ image path alt }}          Returns the HTML that shows the image from
//                                provided path and alt text.
//
//  {{ pages type }}              Returns the slice of pages of
//                                supplied type. If type is empty, all
//                                pages are returned.
//
//  {{ url base }}                Returns the URL based on joining the
//                                site base URL with the supplied URL.
//
//  {{ path . }}                  Returns a path to the page source.
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
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	ttemplate "text/template"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/feeds"
	"github.com/russross/blackfriday/v2"
)

// SupportedFormats contains supported page formats.
var SupportedFormats = []string{".html", ".md"}

// Possible errors, used in tests.
var (
	errFrontmatterSplit        = errors.New("failed to split frontmatter and contents")
	errFrontmatterParse        = errors.New("failed to parse frontmatter")
	errFrontmatterMissing      = errors.New("missing frontmatter")
	errFrontmatterMissingParam = errors.New("missing required frontmatter parameter (title, template, permalink)")
	errFormatUnsupported       = errors.New("format unsupported")
	errInvalidPermalink        = errors.New("invalid permalink")
)

// Logf is a simple printf-like logging function.
type Logf func(format string, args ...any)

// Env is the environment for which site can be built.
type Env string

// Available environments.
const (
	// Everything is included.
	Dev = Env("dev")
	// Drafts are excluded. Also the base URL is used to derive absolute URLs from
	// relative ones.
	Prod = Env("prod")
	// Mostly similar to prod, but drafts are included.
	Staging = Env("staging")
)

// Config represents a build configuration.
type Config struct {
	// Title is the title of the site.
	Title string
	// Author is the name of the author of the site.
	Author string
	// Env is the environment to use when building.
	Env Env
	// BaseURL is the base URL of the site.
	BaseURL *url.URL
	// Src is the directory where to read files from.  If empty, uses the current
	// directory.
	Src string
	// Dst is the directory where to write files.  If empty, uses the build
	// directory.
	Dst string
	// Logf specifies a logger to use. If nil, log.Printf is used.
	Logf Logf
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

	if c.Env == "" {
		c.Env = Dev
	}

	if c.BaseURL == nil {
		c.BaseURL = &url.URL{
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

// Build builds a site based on the provided Config.
func Build(c *Config) error {
	c.setDefaults()

	// Prepare the build context and template functions.
	b := newBuildContext(c)

	// Parse templates and pages.
	if err := filepath.WalkDir(filepath.Join(b.c.Src, "templates"), b.parseTemplates); err != nil {
		return err
	}
	if err := filepath.WalkDir(filepath.Join(b.c.Src, "pages"), b.parsePages); err != nil {
		return err
	}

	// Clean up after previous build.
	if _, err := os.Stat(b.c.Dst); err == nil {
		if err := os.RemoveAll(b.c.Dst); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(b.c.Dst, 0o755); err != nil {
		return err
	}

	// Build pages.
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

	// Build feeds.
	if err := b.buildFeed(); err != nil {
		return err
	}

	// Copy static files.
	if err := filepath.WalkDir(filepath.Join(b.c.Src, "static"), b.copyStatic); err != nil {
		return err
	}

	return nil
}

// Serve builds the site and starts serving it on a provided host:port.
func Serve(c *Config, addr string) error {
	c.setDefaults()

	c.Logf("Performing an initial build...")
	if err := Build(c); err != nil {
		return err
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
		if err := watcher.Add(dir); err != nil {
			return err
		}
	}
	defer watcher.Close()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	c.Logf("Listening on %s://%s...", l.Addr().Network(), l.Addr().String())

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(neuteredFileSystem{http.Dir(c.Dst)}))

	errc := make(chan error, 1)
	s := &http.Server{Handler: mux}
	go func() {
		if err := s.Serve(l); err != nil {
			if err != http.ErrServerClosed {
				errc <- err
			}
		}
	}()
	go func() {
		c.Logf("Started watching for new changes.")
		for event := range watcher.Events {
			if !shouldRebuild(event.Name, event.Op) {
				continue
			}

			c.Logf("Changed %s, rebuilding the site.", event.Name)
			if err := Build(c); err != nil {
				c.Logf("Failed to rebuild the site: %v", err)
			}
		}
	}()

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-exit:
		c.Logf("Received %s, gracefully shutting down...", sig)
	case <-errc:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.Shutdown(ctx)
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

	// Ignore everything else. Rationale:
	//
	//  * chmod: we don't really care about these as they won't affect build
	//   output (unless potentially we no longer can read the file, but we'll go
	//   down that path if it ever becomes a problem).
	//
	//  * rename: will produce a following create event as well, so just listen
	//  for that instead.
	//
	return false
}

// neuteredFileSystem is an implementation of http.FileSystem which prevents
// showing directory listings when using http.FileServer.
type neuteredFileSystem struct {
	fs http.FileSystem
}

// Open implements the http.FileSystem interface.
func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	//lint:ignore SA4006 TODO
	s, err := f.Stat()
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := nfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

type buildContext struct {
	c     *Config
	funcs template.FuncMap

	pages     []*Page
	templates map[string]*template.Template
}

func newBuildContext(c *Config) *buildContext {
	b := &buildContext{
		c:         c,
		templates: make(map[string]*template.Template),
	}

	b.funcs = template.FuncMap{
		"content": func(p *Page) template.HTML {
			return template.HTML(p.contents)
		},
		"formatDate": func(format string, d *date) string {
			return d.Time.Format(format)
		},
		"env": func() string { return string(b.c.Env) },
		"image": func(path, caption string) template.HTML {
			const tmpl = `<img alt="%[2]s" src="%[1]s" loading="lazy"/>`
			s := fmt.Sprintf(tmpl, b.url(path), caption)
			return template.HTML(s)
		},
		"pages": func(typ string) []*Page {
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
		},
		"url":  b.url,
		"path": func(p *Page) string { return p.path },
	}

	return b
}

func (b *buildContext) url(base string) string {
	if b.c.Env == Dev || b.c.BaseURL == nil {
		return base
	}
	u := *b.c.BaseURL
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

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	p := &Page{path: path}
	if err := p.parse(f); err != nil {
		return err
	}
	if !p.Draft || b.c.Env != Prod {
		b.pages = append(b.pages, p)
	}

	return nil
}

// Page represents a site page. The exported fields is the front matter fields.
type Page struct {
	Title       string `json:"title"`        // title: Page title, required.
	Summary     string `json:"summary"`      // summary: Page summary, used in RSS feed, optional.
	Type        string `json:"type"`         // type: Used to distinguish different kinds of pages, page by default.
	Permalink   string `json:"permalink"`    // permalink: Output path for the page, required.
	Date        *date  `json:"date"`         // date: Publication date in the 'year-month-day' format, e.g. 2006-01-02, optional.
	Draft       bool   `json:"draft"`        // draft: Determines whether this page should be not included in production builds, false by default.
	Template    string `json:"template"`     // template: Template that should be used for rendering this page, required.
	ContentOnly bool   `json:"content_only"` // content_only: Determines whether this page should be rendered without header and footer, false by default.

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
	for _, f := range SupportedFormats {
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
		return fmt.Errorf("%s: %w: %v", p.path, errInvalidPermalink, err)
	}
	p.dstPath = p.Permalink
	if !strings.HasSuffix(p.dstPath, ".html") {
		p.dstPath = p.dstPath + "/index.html"
	}
	p.dstPath = path.Clean(p.dstPath)

	return nil
}

var htmlCommentRe = regexp.MustCompile("<!--(.*?)-->")

func (p *Page) build(b *buildContext, tpl *template.Template, w io.Writer) error {
	// We use here text/template, but not html/template because we didn't want to
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
		p.contents = blackfriday.Run(p.contents)
	}

	p.contents = htmlCommentRe.ReplaceAll(p.contents, []byte{})

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, p); err != nil {
		return fmt.Errorf("%s: failed to execute template %q: %w", p.path, p.Template, err)
	}

	_, err = buf.WriteTo(w)
	return err
}

func (b *buildContext) copyStatic(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

	from, err := os.Open(path)
	if err != nil {
		return err
	}
	defer from.Close()

	toPath, err := filepath.Rel(filepath.Join(b.c.Src, "static"), path)
	if err != nil {
		return err
	}
	toPath = filepath.Join(b.c.Dst, toPath)

	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return err
	}
	to, err := os.Create(toPath)
	if err != nil {
		return err
	}
	defer to.Close()

	if _, err := io.Copy(to, from); err != nil {
		return err
	}

	return nil
}

func (b *buildContext) buildFeed() error {
	feed := &feeds.Feed{
		Title:   b.c.Title,
		Link:    &feeds.Link{Href: b.c.BaseURL.String() + "/"},
		Author:  &feeds.Author{Name: b.c.Author},
		Created: time.Now(),
	}

	for _, p := range b.pages {
		if p.Type != "post" {
			continue
		}

		// Exclude drafts from feed in prod.
		if p.Draft && b.c.Env == Prod {
			continue
		}

		pu := *b.c.BaseURL
		pu.Path = path.Join(pu.Path, p.Permalink)

		feed.Items = append(feed.Items, &feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: pu.String()},
			Author:      feed.Author,
			Created:     p.Date.Time,
			Description: p.Summary,
			Content:     string(p.contents),
		})
	}

	bf, err := feed.ToAtom()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.c.Dst, "feed.xml"), []byte(bf), 0o644)
}
