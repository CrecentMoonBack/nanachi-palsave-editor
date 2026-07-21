package icons

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The repository ships no artwork, so every test here has to work on a machine
// that has never run scripts/fetch-icons.sh. They do that by building their own
// icons directory in a temp dir and chdir-ing into it, which makes the working
// tree's real assets/icons irrelevant either way.

// chdir moves into dir for the duration of the test. os.Chdir rather than
// t.Chdir because the module declares go 1.23 and t.Chdir landed in 1.24.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatalf("restoring wd: %v", err)
		}
	})
}

// fakeIcons builds <tmp>/assets/icons containing the named files, plus a
// "secret.webp" one level above it that nothing is ever allowed to reach. It
// chdirs into the temp dir and returns it.
func fakeIcons(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "assets", "icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("inside:"+n), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	// The prize an attacker would be after: a readable file that is a sibling
	// of the icons directory, with a name and extension we would otherwise be
	// perfectly happy to serve.
	if err := os.WriteFile(filepath.Join(root, "assets", "secret.webp"), []byte("SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	chdir(t, root)
	return dir
}

func TestAbsentDirectory(t *testing.T) {
	chdir(t, t.TempDir())

	if got := Dir(); got != "" {
		t.Errorf("Dir() = %q, want empty when no icons directory exists", got)
	}
	if Available() {
		t.Error("Available() = true with no icons directory")
	}
	if got := Count(); got != 0 {
		t.Errorf("Count() = %d, want 0", got)
	}
	if Has("alpaca.webp") {
		t.Error("Has() = true with no icons directory")
	}

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/alpaca.webp", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when the directory is missing", rec.Code)
	}
}

func TestEmptyDirectoryCountsAsUnavailable(t *testing.T) {
	dir := fakeIcons(t)

	if got := Dir(); got != dir {
		t.Errorf("Dir() = %q, want %q", got, dir)
	}
	if Available() {
		t.Error("Available() = true for an empty icons directory")
	}
}

func TestPresentDirectory(t *testing.T) {
	dir := fakeIcons(t, "alpaca.webp", "custom.png")

	if got := Dir(); got != dir {
		t.Errorf("Dir() = %q, want %q", got, dir)
	}
	if !Available() {
		t.Error("Available() = false with two icons present")
	}
	if got := Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
	for _, n := range []string{"alpaca.webp", "custom.png"} {
		if !Has(n) {
			t.Errorf("Has(%q) = false", n)
		}
	}
	if Has("nosuchpal.webp") {
		t.Error("Has() = true for a file that is not there")
	}
}

func TestCountIgnoresNonIcons(t *testing.T) {
	dir := fakeIcons(t, "alpaca.webp")
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "steam.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := Count(); got != 1 {
		t.Errorf("Count() = %d, want 1 (only the .webp)", got)
	}
	// The allow-list is a boundary, not just a MIME table: files we have no
	// content type for are not served at all.
	if Has("README.txt") || Has("steam.svg") || Has("app") {
		t.Error("Has() accepted a file outside the image allow-list")
	}
}

func TestServesFile(t *testing.T) {
	fakeIcons(t, "alpaca.webp")

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/alpaca.webp", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got, want := rec.Body.String(), "inside:alpaca.webp"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "image/webp"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=") {
		t.Errorf("Cache-Control = %q, want a long max-age", got)
	}
}

// TestServesUnderStripPrefix pins the property the traversal defence relies on:
// only the last path element matters, so the GUI may mount the handler either
// bare or behind http.StripPrefix.
func TestServesUnderStripPrefix(t *testing.T) {
	fakeIcons(t, "alpaca.webp")
	h := http.StripPrefix("/icons/", Handler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/alpaca.webp", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 behind StripPrefix", rec.Code)
	}
}

func TestServesPNG(t *testing.T) {
	fakeIcons(t, "custom.png")

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/custom.png", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got, want := rec.Header().Get("Content-Type"), "image/png"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

func TestRejectsNonGET(t *testing.T) {
	fakeIcons(t, "alpaca.webp")

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/icons/alpaca.webp", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// hostilePaths are request paths that must never reach outside the icons
// directory. httptest.NewRequest parses them the same way the server does, so
// URL.Path arrives percent-decoded exactly as it would in production.
var hostilePaths = []string{
	"/icons/../secret.webp",
	"/icons/../../assets/secret.webp",
	"/icons/../../../../../../etc/passwd",
	"/icons/..%2fsecret.webp",
	"/icons/..%252fsecret.webp",
	"/icons/%2e%2e%2fsecret.webp",
	"/icons/%2e%2e/secret.webp",
	"/icons/....//secret.webp",
	"/icons/..;/secret.webp",
	"/icons/..\\secret.webp",
	"/icons/..%5csecret.webp",
	"/icons/....\\\\secret.webp",
	"/../assets/secret.webp",
	"/secret.webp",
	"//secret.webp",
	"/icons/C:/Windows/win.ini",
	"/icons/C:%5CWindows%5Cwin.ini",
	"/icons/%00alpaca.webp",
	"/icons/alpaca.webp%00.txt",
	"/icons/alpaca.webp:$DATA",
	"/icons/.env",
	"/icons/..",
	"/icons/.",
	"/icons/",
	"/",
}

func TestHandlerRejectsTraversal(t *testing.T) {
	fakeIcons(t, "alpaca.webp")
	h := Handler()

	for _, p := range hostilePaths {
		t.Run(p, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://example.test"+p, nil)
			if err != nil {
				// A path Go refuses to parse is refused before it reaches us,
				// which is the outcome we wanted anyway.
				t.Skipf("unparseable as a URL: %v", err)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if body := rec.Body.String(); strings.Contains(body, "SECRET") {
				t.Fatalf("leaked a file outside the icons directory: %q", body)
			}
		})
	}
}

// TestHandlerRejectsTraversalOverTheWire runs the same inputs through a real
// server, so the encoded variants are transported and re-parsed rather than
// being handed straight to the handler as an already-decoded path.
func TestHandlerRejectsTraversalOverTheWire(t *testing.T) {
	fakeIcons(t, "alpaca.webp")

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	client := &http.Client{
		// A redirect that escaped the mount point would be a finding in
		// itself, so refuse to follow one.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, p := range hostilePaths {
		t.Run(p, func(t *testing.T) {
			u, err := url.Parse(srv.URL + p)
			if err != nil {
				t.Skipf("unparseable as a URL: %v", err)
			}
			resp, err := client.Get(u.String())
			if err != nil {
				t.Skipf("request not transportable: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Errorf("status = 200, want a refusal")
			}
			buf := make([]byte, 64)
			n, _ := resp.Body.Read(buf)
			if strings.Contains(string(buf[:n]), "SECRET") {
				t.Fatalf("leaked a file outside the icons directory")
			}
		})
	}
}

// TestHasRejectsTraversal covers the same boundary at the API the GUI calls
// directly. Has must never confirm the existence of anything outside the
// directory, since that alone is an information leak.
func TestHasRejectsTraversal(t *testing.T) {
	fakeIcons(t, "alpaca.webp")

	for _, name := range []string{
		"../secret.webp",
		"..\\secret.webp",
		"../../assets/secret.webp",
		"..",
		".",
		"",
		"/etc/passwd",
		"C:\\Windows\\win.ini",
		"C:secret.webp",
		"alpaca.webp:$DATA",
		"./alpaca.webp",
		".hidden.webp",
		"sub/alpaca.webp",
	} {
		if Has(name) {
			t.Errorf("Has(%q) = true, want false", name)
		}
	}
}

// TestRealIconsDirectory is a smoke test against whatever the developer
// actually has on disk. It asserts consistency, never presence, so it passes
// on a machine that has never fetched the artwork.
func TestRealIconsDirectory(t *testing.T) {
	dir := Dir()
	if dir == "" {
		t.Skip("no icons directory; run scripts/fetch-icons.sh to exercise this")
	}
	n := Count()
	t.Logf("icons directory %s holds %d icons", dir, n)

	if Available() != (n > 0) {
		t.Errorf("Available() = %v but Count() = %d", Available(), n)
	}
	if n == 0 {
		t.Skip("icons directory is empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".webp" {
			continue
		}
		if !Has(e.Name()) {
			t.Errorf("Has(%q) = false for a file that is in the directory", e.Name())
		}
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/icons/"+e.Name(), nil))
		if rec.Code != http.StatusOK {
			t.Errorf("serving %q: status = %d, want 200", e.Name(), rec.Code)
		}
		break
	}
}
