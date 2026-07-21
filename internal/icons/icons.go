// Package icons serves the pal and item artwork the GUI displays.
//
// No artwork ships with this repository — it is Pocketpair's. `internal/paldata`
// maps ids to icon *filenames*; the files themselves are supplied by the user,
// normally by running `scripts/fetch-icons.sh` to populate `assets/icons/`.
// Everything here is therefore optional by design: when the directory is
// missing the accessors report absence and the handler 404s, and the GUI falls
// back to text names. See docs/THIRD_PARTY.md.
//
// The directory is located the same way internal/oodle locates its DLL:
// alongside the executable first, then the working tree, so that `wails dev`
// and a shipped build both find it.
package icons

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// dirName is the layout a release uses: an assets/icons folder beside the
// executable. It is also where fetch-icons.sh writes inside the working tree.
var relDir = filepath.Join("assets", "icons")

// contentTypes is both the MIME table and the allow-list of what may be
// served. Anything else in the directory is treated as not present, so
// pointing the folder at unrelated files cannot turn this into a file server.
//
// The reference artwork is .webp throughout; .png is accepted because a user
// dropping in their own replacements is a reasonable thing to do.
var contentTypes = map[string]string{
	".webp": "image/webp",
	".png":  "image/png",
}

// searchPaths returns the candidate icon directories, in order: alongside the
// executable first (how a release is laid out), then the working tree (how
// `go test` and `wails dev` see it).
func searchPaths() []string {
	var paths []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(dir, relDir), filepath.Join(dir, "icons"))
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths,
			filepath.Join(wd, relDir),
			// from internal/icons during `go test ./...`
			filepath.Join(wd, "..", "..", relDir),
		)
	}
	return paths
}

// Dir returns the resolved icons directory, or "" when none was found.
//
// The result is deliberately not cached. Unlike loading a DLL this is a
// handful of stat calls, and the directory legitimately appears mid-run: a
// user can run fetch-icons.sh while the GUI is open and expect it to notice.
func Dir() string {
	for _, p := range searchPaths() {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

// Available reports whether an icons directory was found and is non-empty.
// An empty directory counts as no artwork: it is what a half-finished fetch
// leaves behind, and the GUI should treat it the same as absent.
func Available() bool { return Count() > 0 }

// Count returns how many icon files are present, for a status line. It counts
// only files this package would actually serve, so a stray README or a
// subdirectory does not inflate the number.
func Count() int {
	dir := Dir()
	if dir == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := contentTypes[strings.ToLower(filepath.Ext(e.Name()))]; ok {
			n++
		}
	}
	return n
}

// Has reports whether a specific icon file exists. filename is a bare name as
// returned by paldata (e.g. "icehorse_dark.webp"); anything carrying a path,
// a drive letter or an unsupported extension is reported absent rather than
// resolved, so a caller cannot reach outside the icons directory through it.
func Has(filename string) bool {
	_, ok := resolve(filename)
	return ok
}

// resolve validates a requested name and returns its full path.
//
// This is the security boundary of the package. The icons directory is flat,
// which makes the rule simple and total: a name is acceptable only if it is a
// single path element with a known image extension. Everything else — "..",
// separators of either flavour, absolute paths, Windows drive-relative names
// and NTFS alternate data streams — is rejected before it ever reaches the
// filesystem, so there is no cleaned-path comparison to get subtly wrong.
func resolve(filename string) (string, bool) {
	dir := Dir()
	if dir == "" || filename == "" {
		return "", false
	}
	// Reject anything that could denote a path rather than a name. ':' covers
	// both "C:foo" and "foo.webp:stream" on Windows.
	if strings.ContainsAny(filename, `/\:`) {
		return "", false
	}
	// "." and ".." are not names, and a leading dot is a hidden file we have
	// no business serving.
	if strings.HasPrefix(filename, ".") {
		return "", false
	}
	if _, ok := contentTypes[strings.ToLower(filepath.Ext(filename))]; !ok {
		return "", false
	}
	full := filepath.Join(dir, filename)
	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		return "", false
	}
	return full, true
}

// Handler serves icon files so the frontend can use plain
// <img src="/icons/foo.webp">. It works whether or not it is wrapped in
// http.StripPrefix: only the last element of the request path is ever
// considered, because the icons directory is flat.
//
// A missing file and a missing directory are both a plain 404 — artwork is
// optional and its absence is not an error worth logging.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// r.URL.Path is already percent-decoded, so "%2e%2e%2f" arrives here
		// as "../". Cleaning against a rooted path collapses it, and taking
		// the base then discards whatever it was trying to climb to; resolve
		// re-checks the result regardless.
		name := path.Base(path.Clean("/" + r.URL.Path))
		full, ok := resolve(name)
		if !ok {
			http.NotFound(w, r)
			return
		}

		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Set before ServeContent so it does not sniff: mime.TypeByExtension
		// does not know .webp on every machine.
		w.Header().Set("Content-Type", contentTypes[strings.ToLower(filepath.Ext(name))])
		// The files are extracted game assets keyed by a name that encodes
		// their content. They never change under a given name.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		http.ServeContent(w, r, name, fi.ModTime(), io.ReadSeeker(f))
	})
}
