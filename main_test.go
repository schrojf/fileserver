package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestServer creates a Server with valid parsed templates for testing.
func newTestServer(t *testing.T, rootDir string) *Server {
	t.Helper()
	srv, err := NewServer(rootDir, 0)
	if err != nil {
		t.Fatalf("newTestServer: %v", err)
	}
	return srv
}

// createTestTree creates a temporary directory tree for testing and returns its path.
//
//	root/
//	  alpha.txt       (13 bytes: "hello, world!")
//	  beta.txt        (5 bytes: "test\n")
//	  charlie.txt     (7 bytes: "charlie")
//	  subdir/
//	    nested.txt    (6 bytes: "nested")
//	    deep/
//	  emptydir/
//	  delta_dir/
//	  zulu_dir/
func createTestTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"subdir",
		"subdir/deep",
		"emptydir",
		"delta_dir",
		"zulu_dir",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	files := map[string]string{
		"alpha.txt":          "hello, world!",
		"beta.txt":           "test\n",
		"charlie.txt":        "charlie",
		"subdir/nested.txt":  "nested",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return root
}

// ---------------------------------------------------------------------------
// 1. TestFormatSize
// ---------------------------------------------------------------------------

func TestFormatSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"512 bytes", 512, "512 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"exact 1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"exact 1 MB", 1048576, "1.0 MB"},
		{"exact 1 GB", 1073741824, "1.0 GB"},
		{"exact 1 TB", 1099511627776, "1.0 TB"},
		{"exact 1 PB", 1125899906842624, "1.0 PB"},
		{"5 MB", 5242880, "5.0 MB"},
		{"117.7 MB", 123456789, "117.7 MB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatSize(tc.input)
			if got != tc.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. TestValidateRootDirectory
// ---------------------------------------------------------------------------

func TestValidateRootDirectory(t *testing.T) {
	t.Parallel()

	t.Run("valid directory", func(t *testing.T) {
		t.Parallel()
		if err := validateRootDirectory(t.TempDir()); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		t.Parallel()
		err := validateRootDirectory(filepath.Join(t.TempDir(), "nonexistent"))
		if err == nil {
			t.Fatal("expected error for non-existent path")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error %q should contain 'does not exist'", err)
		}
	})

	t.Run("path is a file", func(t *testing.T) {
		t.Parallel()
		f := filepath.Join(t.TempDir(), "afile")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := validateRootDirectory(f)
		if err == nil {
			t.Fatal("expected error for file path")
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("error %q should contain 'not a directory'", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. TestNewServer
// ---------------------------------------------------------------------------

func TestNewServer(t *testing.T) {
	t.Parallel()

	t.Run("valid directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srv, err := NewServer(dir, 9090)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if srv.template == nil {
			t.Error("template should not be nil")
		}
		if srv.port != 9090 {
			t.Errorf("port = %d, want 9090", srv.port)
		}
		if !filepath.IsAbs(srv.rootDir) {
			t.Errorf("rootDir %q should be absolute", srv.rootDir)
		}
		abs, _ := filepath.Abs(dir)
		if srv.rootDir != abs {
			t.Errorf("rootDir = %q, want %q", srv.rootDir, abs)
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		t.Parallel()
		_, err := NewServer(filepath.Join(t.TempDir(), "nope"), 0)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("path is a file", func(t *testing.T) {
		t.Parallel()
		f := filepath.Join(t.TempDir(), "file")
		os.WriteFile(f, []byte("x"), 0o644)
		_, err := NewServer(f, 0)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("relative path resolves to absolute", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Use a path with ".." that still resolves to dir
		rel := filepath.Join(dir, "child", "..")
		os.MkdirAll(filepath.Join(dir, "child"), 0o755)
		srv, err := NewServer(rel, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(srv.rootDir, "..") {
			t.Errorf("rootDir %q should not contain '..'", srv.rootDir)
		}
		if !filepath.IsAbs(srv.rootDir) {
			t.Errorf("rootDir %q should be absolute", srv.rootDir)
		}
	})
}

// ---------------------------------------------------------------------------
// 4. TestIsMountPoint
// ---------------------------------------------------------------------------

func TestIsMountPoint(t *testing.T) {
	t.Parallel()

	t.Run("regular directory is not mount point", func(t *testing.T) {
		t.Parallel()
		if isMountPoint(t.TempDir()) {
			t.Error("temp dir should not be a mount point")
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		t.Parallel()
		if isMountPoint(filepath.Join(t.TempDir(), "nope")) {
			t.Error("non-existent path should return false")
		}
	})

	t.Run("root filesystem", func(t *testing.T) {
		t.Parallel()
		// "/" parent is "/" (same path), so device IDs are equal → false.
		// This documents current behavior.
		got := isMountPoint("/")
		if got {
			t.Log("/ reported as mount point (expected on some systems)")
		}
	})
}

// ---------------------------------------------------------------------------
// 5. TestCheckMountPointHealth
// ---------------------------------------------------------------------------

func TestCheckMountPointHealth(t *testing.T) {
	t.Parallel()

	t.Run("readable directory", func(t *testing.T) {
		t.Parallel()
		if err := checkMountPointHealth(t.TempDir()); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		t.Parallel()
		err := checkMountPointHealth(filepath.Join(t.TempDir(), "gone"))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "mount point unhealthy") {
			t.Errorf("error %q should contain 'mount point unhealthy'", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. TestIsPathSafe
// ---------------------------------------------------------------------------

func TestIsPathSafe(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	srv := newTestServer(t, root)

	// Note: isPathSafe uses filepath.Clean on the input before joining with rootDir.
	// This means paths like "/../etc/passwd" get cleaned to "/etc/passwd", then joined
	// as rootDir+"/etc/passwd" which is within rootDir. In production, net/http.ServeMux
	// also cleans URL paths before they reach the handler, so path traversal via ".."
	// is blocked at the HTTP layer. isPathSafe serves as defense-in-depth.
	tests := []struct {
		name string
		path string
		safe bool
	}{
		{"root", "/", true},
		{"normal file", "/file.txt", true},
		{"subdirectory", "/subdir/", true},
		{"nested file", "/subdir/file.txt", true},
		{"deep nesting", "/a/b/c/d", true},
		{"empty string", "", true},
		{"double slash", "//", true},
		{"traversal staying within root", "/subdir/../subdir/file", true},
		// After filepath.Clean, traversal paths resolve within rootDir:
		{"traversal cleaned to root", "/../etc/passwd", true},
		{"multiple traversals cleaned", "/../../../etc/shadow", true},
		{"traversal from subdir cleaned", "/subdir/../../etc/passwd", true},
		{"direct parent cleaned to root", "/..", true},
		{"direct parent slash cleaned", "/../", true},
		{"mixed dots cleaned", "/./../../etc", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := srv.isPathSafe(tc.path)
			if got != tc.safe {
				t.Errorf("isPathSafe(%q) = %v, want %v", tc.path, got, tc.safe)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. TestHandleRequest
// ---------------------------------------------------------------------------

func TestHandleRequest(t *testing.T) {
	t.Parallel()

	root := createTestTree(t)
	srv := newTestServer(t, root)

	tests := []struct {
		name         string
		method       string
		path         string
		wantStatus   int
		bodyContains string
	}{
		{"GET root directory", http.MethodGet, "/", 200, "File Server"},
		{"GET existing file", http.MethodGet, "/alpha.txt", 200, "hello, world!"},
		{"GET subdirectory", http.MethodGet, "/subdir/", 200, "nested.txt"},
		{"GET nested file", http.MethodGet, "/subdir/nested.txt", 200, "nested"},
		{"GET non-existent", http.MethodGet, "/no-such-file", 404, "Not found"},
		{"POST method", http.MethodPost, "/", 405, "Method not allowed"},
		{"PUT method", http.MethodPut, "/", 405, "Method not allowed"},
		{"DELETE method", http.MethodDelete, "/", 405, "Method not allowed"},
		// In production, net/http.ServeMux cleans paths before they reach the handler.
		// With httptest, paths are not cleaned, so isPathSafe passes (it cleans internally)
		// and the resolved path doesn't exist, resulting in 404.
		{"path traversal gets cleaned", http.MethodGet, "/../nonexistent", 404, "Not found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.handleRequest(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.bodyContains) {
				t.Errorf("body should contain %q, got:\n%s", tc.bodyContains, body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. TestHandleDirectory
// ---------------------------------------------------------------------------

func TestHandleDirectory(t *testing.T) {
	t.Parallel()

	t.Run("sorting: dirs first then alphabetical", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		body := rec.Body.String()
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		// Directories: delta_dir, emptydir, subdir, zulu_dir (alphabetical)
		// Files: alpha.txt, beta.txt, charlie.txt (alphabetical)
		positions := map[string]int{
			"delta_dir":   strings.Index(body, "delta_dir"),
			"emptydir":    strings.Index(body, "emptydir"),
			"subdir":      strings.Index(body, "subdir"),
			"zulu_dir":    strings.Index(body, "zulu_dir"),
			"alpha.txt":   strings.Index(body, "alpha.txt"),
			"beta.txt":    strings.Index(body, "beta.txt"),
			"charlie.txt": strings.Index(body, "charlie.txt"),
		}

		for name, pos := range positions {
			if pos == -1 {
				t.Fatalf("%q not found in response body", name)
			}
		}

		// All dirs before all files
		for _, dir := range []string{"delta_dir", "emptydir", "subdir", "zulu_dir"} {
			for _, file := range []string{"alpha.txt", "beta.txt", "charlie.txt"} {
				if positions[dir] >= positions[file] {
					t.Errorf("%q (dir) should appear before %q (file)", dir, file)
				}
			}
		}

		// Dirs sorted alphabetically
		if positions["delta_dir"] >= positions["emptydir"] {
			t.Error("delta_dir should appear before emptydir")
		}
		if positions["emptydir"] >= positions["subdir"] {
			t.Error("emptydir should appear before subdir")
		}
		if positions["subdir"] >= positions["zulu_dir"] {
			t.Error("subdir should appear before zulu_dir")
		}

		// Files sorted alphabetically
		if positions["alpha.txt"] >= positions["beta.txt"] {
			t.Error("alpha.txt should appear before beta.txt")
		}
		if positions["beta.txt"] >= positions["charlie.txt"] {
			t.Error("beta.txt should appear before charlie.txt")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/emptydir/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "This directory is empty") {
			t.Error("expected empty directory message")
		}
	})

	t.Run("response headers", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		ct := rec.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
		}
		cc := rec.Header().Get("Cache-Control")
		if cc != "no-cache, no-store, must-revalidate" {
			t.Errorf("Cache-Control = %q, want no-cache, no-store, must-revalidate", cc)
		}
	})

	t.Run("root has no parent link", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		if strings.Contains(rec.Body.String(), "Back to parent directory") {
			t.Error("root directory should not have parent link")
		}
	})

	t.Run("subdirectory has parent link", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/subdir/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		body := rec.Body.String()
		if !strings.Contains(body, "Back to parent directory") {
			t.Error("subdirectory should have parent link")
		}
		// Parent of /subdir/ should be /
		if !strings.Contains(body, `href="/"`) {
			t.Error("parent link should point to /")
		}
	})

	t.Run("deep nesting parent path", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/subdir/deep/", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		body := rec.Body.String()
		if !strings.Contains(body, "Back to parent directory") {
			t.Error("deep directory should have parent link")
		}
		if !strings.Contains(body, `href="/subdir/"`) {
			t.Error("parent link should point to /subdir/")
		}
	})
}

// ---------------------------------------------------------------------------
// 9. TestHandleFile
// ---------------------------------------------------------------------------

func TestHandleFile(t *testing.T) {
	t.Parallel()

	t.Run("file content matches", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != "hello, world!" {
			t.Errorf("body = %q, want %q", rec.Body.String(), "hello, world!")
		}
	})

	t.Run("Content-Length header", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		cl := rec.Header().Get("Content-Length")
		if cl != "13" {
			t.Errorf("Content-Length = %q, want %q", cl, "13")
		}
	})

	t.Run("Last-Modified header", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		lm := rec.Header().Get("Last-Modified")
		if lm == "" {
			t.Fatal("Last-Modified header is missing")
		}
		_, err := time.Parse(http.TimeFormat, lm)
		if err != nil {
			t.Errorf("Last-Modified %q is not valid HTTP time format: %v", lm, err)
		}
	})

	t.Run("Accept-Ranges header", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		ar := rec.Header().Get("Accept-Ranges")
		if ar != "bytes" {
			t.Errorf("Accept-Ranges = %q, want %q", ar, "bytes")
		}
	})

	t.Run("Content-Type detection for text file", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("Content-Type = %q, want something containing text/plain", ct)
		}
	})

	t.Run("range request", func(t *testing.T) {
		t.Parallel()
		root := createTestTree(t)
		srv := newTestServer(t, root)

		req := httptest.NewRequest(http.MethodGet, "/alpha.txt", nil)
		req.Header.Set("Range", "bytes=0-4")
		rec := httptest.NewRecorder()
		srv.handleRequest(rec, req)

		if rec.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
		}
		body := rec.Body.String()
		if body != "hello" {
			t.Errorf("body = %q, want %q", body, "hello")
		}
	})
}

// ---------------------------------------------------------------------------
// 10. TestServerShutdown
// ---------------------------------------------------------------------------

func TestServerShutdown(t *testing.T) {
	t.Parallel()

	t.Run("nil httpServer", func(t *testing.T) {
		t.Parallel()
		srv := &Server{}
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("running server shutdown", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		srv := newTestServer(t, root)

		mux := http.NewServeMux()
		mux.HandleFunc("/", srv.handleRequest)
		srv.httpServer = &http.Server{
			Addr:    ":0",
			Handler: mux,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.httpServer.ListenAndServe()
		}()

		// Give the server a moment to start
		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown error: %v", err)
		}

		// Verify server returned ErrServerClosed
		if err := <-errCh; err != http.ErrServerClosed {
			t.Errorf("expected ErrServerClosed, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 11. TestServerStartAndShutdown (Integration)
// ---------------------------------------------------------------------------

func TestServerStartAndShutdown(t *testing.T) {
	t.Parallel()

	root := createTestTree(t)
	srv := newTestServer(t, root)

	// Set up httpServer and listener manually to avoid the race on s.httpServer
	// that would occur when calling Start() from a goroutine while Shutdown()
	// reads s.httpServer from the test goroutine.
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRequest)
	srv.httpServer = &http.Server{
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.httpServer.Serve(ln)
	}()

	// Make a real HTTP request to verify the server works end-to-end
	resp, err := http.Get("http://" + addr + "/alpha.txt")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "hello, world!" {
		t.Errorf("body = %q, want %q", string(body), "hello, world!")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	if err := <-errCh; err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 12. TestParentPathCalculation
// ---------------------------------------------------------------------------

func TestParentPathCalculation(t *testing.T) {
	t.Parallel()

	root := createTestTree(t)
	srv := newTestServer(t, root)

	tests := []struct {
		name            string
		path            string
		hasParentLink   bool
		parentLinkHref  string
	}{
		{"root has no parent", "/", false, ""},
		{"subdir parent is root", "/subdir/", true, `href="/"`},
		{"deep dir parent is subdir", "/subdir/deep/", true, `href="/subdir/"`},
		{"delta_dir parent is root", "/delta_dir/", true, `href="/"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.handleRequest(rec, req)

			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200 for path %s", rec.Code, tc.path)
			}

			body := rec.Body.String()
			hasLink := strings.Contains(body, "Back to parent directory")

			if tc.hasParentLink && !hasLink {
				t.Errorf("expected parent link for path %s", tc.path)
			}
			if !tc.hasParentLink && hasLink {
				t.Errorf("unexpected parent link for path %s", tc.path)
			}
			if tc.parentLinkHref != "" && !strings.Contains(body, tc.parentLinkHref) {
				t.Errorf("expected %s in body for path %s", tc.parentLinkHref, tc.path)
			}
		})
	}
}

// Ensure fmt is used (for error messages in subtests if needed).
var _ = fmt.Sprintf
