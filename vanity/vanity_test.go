package vanity

import (
	"context"
	"encoding/json"
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
		CloneURL:    filepath.Join("vanity", "testdata", "nogomod.bundle"),
		Owner:       &owner{Login: "example"},
	},
	{
		Name:        "nothing",
		URL:         "https://api.github.com/repos/example/nothing",
		Private:     false,
		Description: "Package nothing does nothing.",
		Archived:    false,
		CloneURL:    filepath.Join("vanity", "testdata", "nothing.bundle"),
		Owner:       &owner{Login: "example"},
	},
}

// TODO: maybe generate this from Git bundle?
var filesForRepo = map[string][]file{
	"nogomod": []file{
		{Path: "README.md"},
	},
	"nothing": []file{
		{Path: "go.mod"},
		{Path: "nothing.go"},
	},
}

func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
		log.Fatalf("Changing working directory failed: %v", err)
	}
	os.Exit(m.Run())
}

func TestBuild(t *testing.T) {
	c := &Config{
		Dir:         t.TempDir(),
		GitHubToken: githubToken,
		Logf:        t.Logf,
		ImportRoot:  "example.com",
		HTTPClient:  testutil.MockHTTPClient(t, testHandler(t)),
	}
	if err := Build(context.Background(), c); err != nil {
		t.Fatal(err)
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
