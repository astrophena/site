// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package site

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"go.astrophena.name/base/testutil"
	"go.astrophena.name/base/txtar"

	"github.com/fsnotify/fsnotify"
)

var update = flag.Bool("update", false, "update golden files in testdata")

func TestBuild(t *testing.T) {
	testutil.RunGolden(t, "testdata/*.txtar", func(t *testing.T, match string) []byte {
		tca, err := txtar.ParseFile(match)
		if err != nil {
			t.Fatal(err)
		}

		srcDir, dstDir := t.TempDir(), t.TempDir()
		testutil.ExtractTxtar(t, tca, srcDir)

		if err := Build(&Config{
			Src:         srcDir,
			Dst:         dstDir,
			Logf:        t.Logf,
			feedCreated: time.Date(2023, time.December, 8, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}

		return testutil.BuildTxtar(t, dstDir)
	}, *update)
}

func TestServe(t *testing.T) {
	// Find a free port for us.
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("Failed to find a free port: %v", err)
	}
	addr := fmt.Sprintf("localhost:%d", port)

	var wg sync.WaitGroup

	ready := make(chan struct{})
	serveReadyHook = func() {
		ready <- struct{}{}
	}
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := Serve(ctx, &Config{
			Src:  "../..",
			Dst:  t.TempDir(),
			Logf: t.Logf,
		}, addr); err != nil {
			errCh <- err
		}
	}()

	// Wait until the server is ready.
	select {
	case err := <-errCh:
		t.Fatalf("Test server crashed during startup or runtime: %v", err)
	case <-ready:
	}

	// Make some HTTP requests.
	urls := []struct {
		url        string
		wantStatus int
	}{
		{url: "/", wantStatus: http.StatusOK},
		{url: "/watched", wantStatus: http.StatusOK},
		{url: "/404", wantStatus: http.StatusOK},
		{url: "/does-not-exist", wantStatus: http.StatusNotFound},
		{url: "/icons/", wantStatus: http.StatusNotFound},
	}

	for _, u := range urls {
		req, err := http.Get("http://" + addr + u.url)
		if err != nil {
			t.Fatal(err)
		}
		if req.StatusCode != u.wantStatus {
			t.Fatalf("GET %s: want status code %d, got %d", u.url, u.wantStatus, req.StatusCode)
		}
	}

	// Try to gracefully shutdown the server.
	cancel()
	// Wait until the server shuts down.
	wg.Wait()
	// See if the server failed to shutdown.
	select {
	case err := <-errCh:
		t.Fatalf("Test server crashed during shutdown: %v", err)
	default:
	}
}

// getFreePort asks the kernel for a free open port that is ready to use.
// Copied from
// https://github.com/phayes/freeport/blob/74d24b5ae9f58fbe4057614465b11352f71cdbea/freeport.go.
func getFreePort() (port int, err error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestShouldRebuild(t *testing.T) {
	cases := map[string]struct {
		path string
		op   fsnotify.Op
		want bool
	}{
		"macOS garbage":   {".DS_Store", fsnotify.Create, false},
		"vim temp file":   {"lololol/4913", fsnotify.Write, false},
		"vim backup file": {"pages/hello.md~", fsnotify.Create, false},
		"file creation":   {"pages/hello.md", fsnotify.Create, true},
		"file removal":    {"pages/hello.md", fsnotify.Remove, true},
		"file write":      {"pages/hello.md", fsnotify.Write, true},
		"ignore chmod":    {"pages/hello.md", fsnotify.Chmod, false},
		"ignore rename":   {"pages/hello.md", fsnotify.Rename, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := shouldRebuild(tc.path, tc.op)
			if got != tc.want {
				t.Fatalf("shouldRebuild(%q, %+v): want %v, got %v", tc.path, tc.op, tc.want, got)
			}
		})
	}
}

func TestStripComments(t *testing.T) {
	b := newBuildContext(&Config{})
	tpl := template.Must(template.New("test").Funcs(b.funcs).Parse(`{{ content . }}`))

	const content = `<!-- prettier-ignore-start -->
{
  "title": "Foo",
  "template": "layout",
  "permalink": "/"
}
<!-- prettier-ignore-end -->

Foo.

<!-- Some comment. -->
<!-- LOL. -->
`

	const strippedContent = "<p>Foo.</p>"

	p := &Page{path: "foo.md"}
	if err := p.parse(strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := p.build(b, tpl, &buf); err != nil {
		t.Fatal(err)
	}

	// Don't care about whitespace.
	got := strings.TrimSpace(buf.String())
	testutil.AssertEqual(t, got, strippedContent)
}

func TestPage(t *testing.T) {
	cases := map[string]struct {
		name, content string
		wantErr       error
		wantType      string
	}{
		"valid frontmatter": {
			name: "foo.md",
			content: `{
  "title": "Foo",
  "template": "layout",
  "permalink": "/"
}

Foo.
`,
		},
		"no frontmatter": {
			name:    "bar.md",
			content: "Hello, world!",
			wantErr: errFrontmatterMissing,
		},
		"invalid frontmatter (missing title)": {
			name: "invalid.md",
			content: `{
  "template": "layout",
  "permalink": "/"
}

Bar.
`,
			wantErr: errFrontmatterMissingParam,
		},
		"unsupported format": {
			name:    "unsupported.rst",
			content: "Sample text.",
			wantErr: errFormatUnsupported,
		},
		"invalid permalink": {
			name: "permalink.md",
			content: `{
  "title": "Foo",
  "template": "layout",
  "permalink": "dwd/"
}

Test.
`,
			wantErr: errPermalinkInvalid,
		},
		"default type": {
			name: "default-type.md",
			content: `{
  "title": "Foo",
  "template": "layout",
  "permalink": "/"
}

Test.
`,
			wantType: "page",
		},
		"blog type": {
			name: "type-blog.md",
			content: `{
  "title": "Foo",
  "template": "test",
  "type": "blog",
  "permalink": "/blog/test"
}

Test
`,
			wantType: "blog",
		},
		"modeline comment": {
			name: "modeline-comment.html",
			content: `<!-- vim: set ft=gotplhtml: -->
{
  "title": "Foo",
  "template": "test",
  "permalink": "/test"
}

<p>Test!</p>
`,
		},
		"invalid frontmatter (JSON)": {
			name: "invalid-frontmatter.html",
			content: `{
	"title": 0
}

<p>test</p>
`,
			wantErr: errFrontmatterParse,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &Page{path: tc.name}
			err := p.parse(strings.NewReader(tc.content))

			// Don't use && because we want to trap all cases where err is
			// nil.
			if err == nil {
				if tc.wantErr != nil {
					t.Fatalf("must fail with error: %v", tc.wantErr)
				}
			}

			if err != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error: %v", err)
			}

			if tc.wantType != "" && p.Type != tc.wantType {
				t.Fatalf("wanted type %s, but got %s", tc.wantType, p.Type)
			}
		})
	}
}

func TestURLTemplateFunc(t *testing.T) {
	bu := &url.URL{
		Scheme: "https",
		Host:   "example.com",
	}
	cases := map[string]struct {
		c    *Config
		in   string
		want string
	}{
		"env dev (base URL set)": {
			c: &Config{
				BaseURL: bu,
			},
			in:   "/test",
			want: "/test",
		},
		"env prod (base URL not set)": {
			c: &Config{
				Prod: true,
			},
			in:   "/lol",
			want: "/lol",
		},
		"env prod (base URL set)": {
			c: &Config{
				BaseURL: bu,
				Prod:    true,
			},
			in:   "/hello",
			want: "https://example.com/hello",
		},
		"single slash": {
			c:    &Config{},
			in:   "/",
			want: "/",
		},
		"full url": {
			c:    &Config{},
			in:   "https://go.astrophena.name",
			want: "https://go.astrophena.name",
		},
	}
	b := &buildContext{}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b.c = tc.c
			got := b.url(tc.in)
			testutil.AssertEqual(t, got, tc.want)
		})
	}
}

func TestNavLinkTemplateFunc(t *testing.T) {
	cases := map[string]struct {
		c        *Config
		p        *Page
		title    string
		iconName string
		path     string
		want     string
	}{
		"Vanity is true": {
			c: &Config{
				BaseURL: &url.URL{
					Scheme: "https",
					Host:   "go.astrophena.name",
				},
				Vanity: true,
			},
			p:        &Page{Permalink: "/hello"},
			title:    "Hello, world!",
			iconName: "hello",
			path:     "/hello",
			want:     "<a href=\"https://astrophena.name/hello\">\n<svg class=\"icon\" aria-hidden=\"true\">\n  <use xlink:href=\"/icons/sprite.svg#icon-hello\"/>\n</svg>Hello, world!</a>",
		},
	}

	b := &buildContext{}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b.c = tc.c
			b.c.setDefaults()

			got := b.navLink(tc.p, tc.title, tc.iconName, tc.path)
			testutil.AssertEqual(t, string(got), tc.want)
		})
	}
}
