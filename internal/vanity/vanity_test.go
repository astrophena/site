// © 2024 Ilya Mateyko. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE.md file.

package vanity

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.astrophena.name/base/testutil"
)

const githubToken = "superdupersecret"

var repos = []repo{
	{
		Name:        "nogomod",
		URL:         "https://api.github.com/repos/example/nogomod",
		Private:     false,
		Description: "Not a Go module.",
		Archived:    false,
		CloneURL:    filepath.Join("internal", "vanity", "testdata", "nogomod.bundle"),
		Owner:       &owner{Login: "example"},
	},
	{
		Name:        "noroot",
		URL:         "https://api.github.com/repos/example/noroot",
		Private:     false,
		Description: "Doesn't have root package.",
		Archived:    false,
		CloneURL:    filepath.Join("internal", "vanity", "testdata", "noroot.bundle"),
		Owner:       &owner{Login: "example"},
	},
	{
		Name:        "nothing",
		URL:         "https://api.github.com/repos/example/nothing",
		Private:     false,
		Description: "Package nothing does nothing.",
		Archived:    false,
		CloneURL:    filepath.Join("internal", "vanity", "testdata", "nothing.bundle"),
		Owner:       &owner{Login: "example"},
	},
	{
		Name:        "base",
		URL:         "https://api.github.com/repos/example/base",
		Private:     false,
		Description: "Package base does base.",
		Archived:    false,
		CloneURL:    filepath.Join("internal", "vanity", "testdata", "base.bundle"),
		Owner:       &owner{Login: "example"},
	},
	{
		Name:        "internalonlyrepo",
		URL:         "https://api.github.com/repos/example/internalonlyrepo",
		Private:     false,
		Description: "Repo with only internal packages.",
		Archived:    false,
		CloneURL:    filepath.Join("internal", "vanity", "testdata", "internalonlyrepo.bundle"),
		Owner:       &owner{Login: "example"},
	},
}

// TODO: maybe generate this from Git bundle?
var filesForRepo = map[string][]file{
	"nogomod": {
		{Path: "README.md"},
	},
	"noroot": {
		{Path: "go.mod"},
		{Path: "hello/hello.go"},
	},
	"nothing": {
		{Path: "go.mod"},
		{Path: "nothing.go"},
	},
	"base": {
		{Path: "LICENSE.md"},
		{Path: "README.md"},
		{Path: "base.go"},
		{Path: "go.mod"},
		{Path: "testutil/testutil.go"},
		{Path: "txtar/txtar.go"},
	},
	"internalonlyrepo": {
		{Path: "go.mod"},
		{Path: "internal/pkg1/pkg1.go"},
		{Path: "internal/deeper/pkg2/pkg2.go"},
	},
}

var inspect = flag.Bool("inspect", false, "print location of test site for inspection")

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		log.Fatalf("Changing working directory failed: %v", err)
	}
	flag.Parse()
	os.Exit(m.Run())
}

func TestBuild(t *testing.T) {
	var dir string

	if *inspect {
		var err error
		dir, err = os.MkdirTemp("", "vanity-test-build")
		if err != nil {
			t.Fatal(err)
		}
	} else {
		dir = t.TempDir()
	}

	c := &Config{
		Dir:         dir,
		GitHubToken: githubToken,
		Logf:        t.Logf,
		ImportRoot:  "example.com",
		HTTPClient:  testutil.MockHTTPClient(testHandler(t)),
	}
	if err := Build(t.Context(), c); err != nil {
		t.Fatal(err)
	}

	// Check some required files.
	for _, f := range []string{
		"404.html",
		"index.html",
		"css/godoc.css",
		"css/main.css",
	} {
		wantFile(t, filepath.Join(dir, f))
	}

	// Check that internalonlyrepo is missing from the index page.
	indexPage, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(indexPage), "internalonlyrepo") {
		t.Errorf("internalonlyrepo should be missing from the index page, got:\n\t%s", indexPage)
	}

	// Check that internalonlyrepo contains only a placeholder.
	internalOnlyPage, err := os.ReadFile(filepath.Join(dir, "internalonlyrepo.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(internalOnlyPage), "This repository does not contain any importable packages.") {
		t.Errorf("internalonlyrepo page should contain only a placeholder, got:\n\t%s", internalOnlyPage)
	}

	if *inspect {
		fmt.Fprintf(os.Stderr, "%s\n", dir)
	}
}

func wantFile(t *testing.T, path string) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Errorf("file %q doesn't exist", path)
	} else if err != nil {
		t.Errorf("checking existence of file %q failed: %v", path, err)
	}
}

func testHandler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("api.github.com/user/repos", func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), githubToken)
		respondJSON(t, w, repos)
	})
	mux.HandleFunc("api.github.com/repos/{owner}/{repo}/contents", func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, r.PathValue("owner"), "example")
		repo := r.PathValue("repo")
		files, ok := filesForRepo[repo]
		if !ok {
			http.NotFound(w, r)
			return
		}
		respondJSON(t, w, files)
	})
	return mux
}

func respondJSON(t *testing.T, w http.ResponseWriter, data any) {
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}

func TestReplaceRelLinks(t *testing.T) {
	c := &Config{
		ImportRoot: "go.astrophena.name",
	}

	cases := map[string]struct {
		in   string
		want string
		pkg  *pkg
	}{
		"no links": {
			in: `
<h1>Package docs</h1>
<p>This is a package.</p>
`,
			want: `
<h1>Package docs</h1>
<p>This is a package.</p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
		"relative link within module": {
			in: `
<h1>Package docs</h1>
<p>This package uses <a href="../txtar">txtar</a>.</p>
`,
			want: `
<h1>Package docs</h1>
<p>This package uses <a href="/base/txtar">txtar</a>.</p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
		"multiple relative links within module": {
			in: `
<h1>Package docs</h1>
<p>This package uses <a href="../txtar">txtar</a> and <a href="../foo/bar">foo/bar</a>.</p>
`,
			want: `
<h1>Package docs</h1>
<p>This package uses <a href="/base/txtar">txtar</a> and <a href="/base/foo/bar">foo/bar</a>.</p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
		"external link": {
			in: `
<h1>Package docs</h1>
<p>This package uses <a href="https://example.com">example.com</a>.</p>
`,
			want: `
<h1>Package docs</h1>
<p>This package uses <a href="https://example.com">example.com</a>.</p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
		"mixed links": {
			in: `
<h1>Package docs</h1>
<p>
	This package uses
	<a href="../txtar">txtar</a>,
	<a href="internal/workshop">internal/workshop</a>,
	<a href="../foo/bar/..#Logf">foo/bar</a>, and
	<a href="https://pkg.go.dev/builtin#string">example.com</a>.
</p>
`,
			want: `
<h1>Package docs</h1>
<p>
	This package uses
	<a href="/base/txtar">txtar</a>,
	<a href="/base/testutil/internal/workshop">internal/workshop</a>,
	<a href="/base/foo#Logf">foo/bar</a>, and
	<a href="https://pkg.go.dev/builtin#string">example.com</a>.
</p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.pkg.FullDoc = tc.in
			tc.pkg.replaceRelLinks(c)
			testutil.AssertEqual(t, tc.pkg.FullDoc, tc.want)
		})
	}
}
