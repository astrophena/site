// © 2022 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE.md file.

package site

import (
	"bytes"
	"errors"
	"html/template"
	"net/url"
	"strings"
	"testing"
)

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

	p := &Page{name: "foo.md"}
	if err := p.parse(strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := p.build(b, tpl, &buf); err != nil {
		t.Fatal(err)
	}

	// Check if all comments has been removed.
	comments := htmlCommentRe.FindAll(buf.Bytes(), -1)
	if len(comments) > 0 {
		t.Fatalf("not all comments has been stripped, %d remains", len(comments))
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
			p := &Page{name: tc.name}
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
