// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package site

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestBuild(t *testing.T) {
	if err := Build(&Config{Dst: t.TempDir()}); err != nil {
		t.Fatal(err)
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

	go func() {
		if err := Serve(ctx, &Config{
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
	urls := []string{"/", "/blocklist.txt", "/watched/"}
	for _, u := range urls {
		req, err := http.Get("http://" + addr + u)
		if err != nil {
			t.Fatal(err)
		}
		if req.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: want status code 200, got %d", u, req.StatusCode)
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
	if got != strippedContent {
		t.Fatalf("want %q, got %q", strippedContent, got)
	}
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
		"env dev (base URL not set, trailing slash retained)": {
			c:    &Config{},
			in:   "/hello/",
			want: "/hello/",
		},
		"env prod (base URL set, trailing slash retained": {
			c: &Config{
				BaseURL: bu,
				Prod:    true,
			},
			in:   "/hello/",
			want: "https://example.com/hello/",
		},
		"env prod (base URL not set, trailing slash retained": {
			c: &Config{
				Prod: true,
			},
			in:   "/lol/",
			want: "/lol/",
		},
		"single slash": {
			c:    &Config{},
			in:   "/",
			want: "/",
		},
	}
	b := &buildContext{}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b.c = tc.c
			got := b.url(tc.in)

			if got != tc.want {
				t.Fatalf("got %q, but want %q", got, tc.want)
			}
		})
	}
}
