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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.astrophena.name/base/testutil"
)

const githubToken = "superdupersecret"

var repos = []repo{
	{
		Name:          "nogomod",
		URL:           "https://api.github.com/repos/example/nogomod",
		Private:       false,
		Description:   "Not a Go module.",
		Archived:      false,
		CloneURL:      filepath.Join("internal", "vanity", "testdata", "nogomod.bundle"),
		DefaultBranch: "master",
		Owner:         &owner{Login: "example"},
	},
	{
		Name:          "noroot",
		URL:           "https://api.github.com/repos/example/noroot",
		Private:       false,
		Description:   "Doesn't have root package.",
		Archived:      false,
		CloneURL:      filepath.Join("internal", "vanity", "testdata", "noroot.bundle"),
		DefaultBranch: "master",
		Owner:         &owner{Login: "example"},
	},
	{
		Name:          "nothing",
		URL:           "https://api.github.com/repos/example/nothing",
		Private:       false,
		Description:   "Package nothing does nothing.",
		Archived:      false,
		CloneURL:      filepath.Join("internal", "vanity", "testdata", "nothing.bundle"),
		DefaultBranch: "master",
		Owner:         &owner{Login: "example"},
	},
	{
		Name:          "base",
		URL:           "https://api.github.com/repos/example/base",
		Private:       false,
		Description:   "Package base does base.",
		Archived:      false,
		CloneURL:      filepath.Join("internal", "vanity", "testdata", "base.bundle"),
		DefaultBranch: "master",
		Owner:         &owner{Login: "example"},
	},
	{
		Name:          "internalonlyrepo",
		URL:           "https://api.github.com/repos/example/internalonlyrepo",
		Private:       false,
		Description:   "Repo with only internal packages.",
		Archived:      false,
		CloneURL:      filepath.Join("internal", "vanity", "testdata", "internalonlyrepo.bundle"),
		DefaultBranch: "master",
		Owner:         &owner{Login: "example"},
	},
}

var secondPageRepo = repo{
	Name:          "page2repo",
	URL:           "https://api.github.com/repos/example/page2repo",
	Private:       false,
	Description:   "Repo returned on the second page.",
	Archived:      false,
	CloneURL:      filepath.Join("internal", "vanity", "testdata", "nothing.bundle"),
	DefaultBranch: "master",
	Owner:         &owner{Login: "example"},
}

// TODO: Derive this from the Git bundles to keep test fixtures in sync.
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
	"page2repo": {
		{Path: "go.mod"},
		{Path: "nothing.go"},
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
		ImportRoot:  "example.com",
		HTTPClient:  testutil.MockHTTPClient(testHandler(t)),
		Concurrency: 2,
	}
	if err := Build(t.Context(), c); err != nil {
		t.Fatal(err)
	}

	// Verify that required output files were generated.
	for _, f := range []string{
		"404.html",
		"index.html",
	} {
		wantFile(t, filepath.Join(dir, f))
	}

	// Verify that internal-only repositories are omitted from the index.
	indexPage, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(indexPage), "internalonlyrepo") {
		t.Errorf("internalonlyrepo should be missing from the index page, got:\n\t%s", indexPage)
	}

	// Verify that internal-only repositories render the placeholder page.
	internalOnlyPage, err := os.ReadFile(filepath.Join(dir, "internalonlyrepo.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(internalOnlyPage), "This module does not contain any importable packages.") {
		t.Errorf("internalonlyrepo page should contain only a placeholder, got:\n\t%s", internalOnlyPage)
	}

	if *inspect {
		fmt.Fprintf(os.Stderr, "%s\n", dir)
	}
}

func TestBuildWithRepoCacheDir(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "repo-cache")
	for range 2 {
		dir := t.TempDir()
		c := &Config{
			Dir:          dir,
			GitHubToken:  githubToken,
			ImportRoot:   "example.com",
			HTTPClient:   testutil.MockHTTPClient(testHandler(t)),
			RepoCacheDir: cacheDir,
			Concurrency:  2,
		}
		if err := Build(t.Context(), c); err != nil {
			t.Fatal(err)
		}
		wantFile(t, filepath.Join(dir, "index.html"))
	}

	wantFile(t, filepath.Join(cacheDir, "base", ".git"))
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

func TestListReposPagination(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), githubToken)

		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page == 0 {
			page = 1
		}
		perPage, err := strconv.Atoi(r.URL.Query().Get("per_page"))
		if err != nil || perPage == 0 {
			perPage = 30
		}

		switch page {
		case 1:
			if perPage != 100 {
				t.Fatalf("per_page = %d, want 100", perPage)
			}
			var fullPage []repo
			for i := 0; i < 100; i++ {
				fullPage = append(fullPage, repo{
					Name:          fmt.Sprintf("page1repo-%03d", i),
					URL:           fmt.Sprintf("https://api.github.com/repos/example/page1repo-%03d", i),
					Description:   "Pagination filler.",
					CloneURL:      filepath.Join("internal", "vanity", "testdata", "nothing.bundle"),
					DefaultBranch: "master",
					Owner:         &owner{Login: "example"},
				})
			}
			respondJSON(t, w, fullPage)
		case 2:
			respondJSON(t, w, []repo{secondPageRepo})
		default:
			respondJSON(t, w, []repo{})
		}
	})

	c := &Config{
		GitHubToken: githubToken,
		HTTPClient:  testutil.MockHTTPClient(handler),
	}

	got, err := listRepos(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 101 {
		t.Fatalf("listRepos() returned %d repos, want 101", len(got))
	}
	if got[0].Name != "page1repo-000" {
		t.Fatalf("first repo = %q, want %q", got[0].Name, "page1repo-000")
	}
	if got[len(got)-1].Name != secondPageRepo.Name {
		t.Fatalf("last repo = %q, want %q", got[len(got)-1].Name, secondPageRepo.Name)
	}
}

func TestGenerateDocContinuesAfterMissingPackage(t *testing.T) {
	dir := t.TempDir()
	cloneDir := filepath.Join(dir, "repo")
	clone := exec.CommandContext(t.Context(), "git", "clone", "--depth=1", filepath.Join("internal", "vanity", "testdata", "base.bundle"), cloneDir)
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("cloning test bundle failed: %v\n%s", err, out)
	}

	r := &repo{
		Name: "base",
		Dir:  cloneDir,
		Pkgs: []*pkg{
			{ImportPath: "example.com/base/missing"},
			{ImportPath: "example.com/base/testutil"},
		},
	}

	if err := r.generateDoc(t.Context(), &Config{ImportRoot: "example.com"}); err != nil {
		t.Fatal(err)
	}

	if r.Pkgs[1].BasePath != "base/testutil" {
		t.Fatalf("BasePath = %q, want %q", r.Pkgs[1].BasePath, "base/testutil")
	}
	if r.Pkgs[1].FullDoc == "" {
		t.Fatal("expected documentation for existing package to be populated")
	}
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
		"fragment-only link": {
			in: `
<h1>Package docs</h1>
<p><a href="#hdr-Example">Example</a></p>
`,
			want: `
<h1>Package docs</h1>
<p><a href="#hdr-Example">Example</a></p>
`,
			pkg: &pkg{
				ImportPath: "go.astrophena.name/base/testutil",
			},
		},
		"mailto link": {
			in: `
<h1>Package docs</h1>
<p><a href="mailto:hello@example.com">Email</a></p>
`,
			want: `
<h1>Package docs</h1>
<p><a href="mailto:hello@example.com">Email</a></p>
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

func TestModifyHTML(t *testing.T) {
	p := &pkg{
		Name:       "main",
		ImportPath: "go.astrophena.name/cmd/tool",
		FullDoc: `
<html><body>
  <h2 id="pkg-overview">Overview</h2>
  <h3 id="hdr-Usage">Usage & setup</h3>
  <h3 id="hdr-Flags">Flags</h3>
</body></html>
`,
	}

	if err := p.modifyHTML(&Config{ImportRoot: "go.astrophena.name"}); err != nil {
		t.Fatal(err)
	}

	wantOrder := []string{
		`<p>Install this program:</p><pre><code>$ go install go.astrophena.name/cmd/tool@latest</code></pre>`,
		`<h3>Contents</h3><ul>`,
		`<li><a href="#hdr-Usage">Usage &amp; setup</a></li>`,
		`<li><a href="#hdr-Flags">Flags</a></li>`,
	}
	for _, want := range wantOrder {
		if !strings.Contains(p.FullDoc, want) {
			t.Fatalf("modified HTML does not contain %q:\n%s", want, p.FullDoc)
		}
	}

	installPos := strings.Index(p.FullDoc, wantOrder[0])
	tocPos := strings.Index(p.FullDoc, wantOrder[1])
	if installPos == -1 || tocPos == -1 || installPos > tocPos {
		t.Fatalf("install snippet should appear before TOC, got:\n%s", p.FullDoc)
	}
}

func TestIsInternalImportPath(t *testing.T) {
	cases := map[string]bool{
		"internal":                          true,
		"internal/pkg":                      true,
		"example.com/mod/pkg/internal":      true,
		"example.com/mod/internal/pkg":      true,
		"example.com/mod/internaltools":     false,
		"example.com/mod/foo/internaltools": false,
		"example.com/mod/public/pkg":        false,
		"internaltools/pkg":                 false,
	}

	for importPath, want := range cases {
		t.Run(importPath, func(t *testing.T) {
			if got := isInternalImportPath(importPath); got != want {
				t.Fatalf("isInternalImportPath(%q) = %v, want %v", importPath, got, want)
			}
		})
	}
}

func TestHasOnlyInternalPackages(t *testing.T) {
	tests := map[string]struct {
		pkgs []*pkg
		want bool
	}{
		"none": {
			pkgs: nil,
			want: false,
		},
		"all internal": {
			pkgs: []*pkg{
				{ImportPath: "example.com/mod/internal/pkg1"},
				{ImportPath: "example.com/mod/internal/deeper/pkg2"},
			},
			want: true,
		},
		"mixed": {
			pkgs: []*pkg{
				{ImportPath: "example.com/mod/internal/pkg1"},
				{ImportPath: "example.com/mod/public"},
			},
			want: false,
		},
		"internaltools is public": {
			pkgs: []*pkg{
				{ImportPath: "example.com/mod/internaltools"},
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := hasOnlyInternalPackages(tc.pkgs); got != tc.want {
				var importPaths []string
				for _, pkg := range tc.pkgs {
					importPaths = append(importPaths, pkg.ImportPath)
				}
				t.Fatalf("hasOnlyInternalPackages(%v) = %v, want %v", importPaths, got, tc.want)
			}
		})
	}
}
