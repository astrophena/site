// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package site

import (
	"bytes"
	"errors"
	"html/template"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func TestBuild(t *testing.T) {
	tmp, err := os.MkdirTemp("", "site-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	if err := Build(&Config{Dst: tmp}); err != nil {
		t.Fatal(err)
	}
}

func TestServe(t *testing.T) {
	_, runServer := os.LookupEnv("SITE_RUN_SERVER")
	if runServer {
		// Signalize to a parent process that we are ready.
		serveReadyHook = func() {
			ppid := os.Getppid()
			proc, err := os.FindProcess(ppid)
			if err != nil {
				t.Fatalf("os.FindProcess(): %v", err)
			}
			if err := proc.Signal(syscall.SIGUSR1); err != nil {
				t.Fatalf("Failed to send a signal: %v", err)
			}
		}
		if err := Serve(&Config{
			Dst: t.TempDir(),
		}, "localhost:0"); err != nil {
			t.Fatal(err)
		}
		return
	}

	var (
		buf bytes.Buffer
		wg  sync.WaitGroup
	)

	// Start the server in a subprocess.
	server := exec.Command(os.Args[0], "-test.run=TestServe")
	server.Stderr = &buf
	server.Env = append(os.Environ(), "SITE_RUN_SERVER=1")
	go func() {
		wg.Add(1)
		if err := server.Run(); err != nil {
			t.Fatalf("Test server crashed: %v", err)
		}
		wg.Done()
	}()

	// Wait until the server is ready.
	ready := make(chan os.Signal, 1)
	signal.Notify(ready, syscall.SIGUSR1)
	<-ready

	// Try to gracefully shutdown the server.
	if err := server.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send a signal to running test server: %v", err)
	}

	// Wait until the server successfully shuts down.
	wg.Wait()
	t.Logf("Test server output:\n%s", buf.String())
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
			wantErr: errInvalidPermalink,
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
				Env:     Dev,
				BaseURL: bu,
			},
			in:   "/test",
			want: "/test",
		},
		"env prod (base URL not set)": {
			c: &Config{
				Env: Prod,
			},
			in:   "/lol",
			want: "/lol",
		},
		"env prod (base URL set)": {
			c: &Config{
				Env:     Prod,
				BaseURL: bu,
			},
			in:   "/hello",
			want: "https://example.com/hello",
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
