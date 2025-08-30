package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	SizeStr string
	ModStr  string
}

type PageData struct {
	Title       string
	CurrentPath string
	ParentPath  string
	Files       []FileInfo
	Error       string
}

type Server struct {
	rootDir    string
	port       int
	template   *template.Template
	httpServer *http.Server
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func NewServer(rootDir string, port int) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %v", err)
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Verify the root directory exists and is accessible
	if err := validateRootDirectory(absRoot); err != nil {
		return nil, err
	}

	return &Server{
		rootDir:  absRoot,
		port:     port,
		template: tmpl,
	}, nil
}

func validateRootDirectory(rootDir string) error {
	info, err := os.Stat(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("root directory does not exist: %s", rootDir)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied accessing root directory: %s", rootDir)
		}
		return fmt.Errorf("cannot access root directory %s: %v", rootDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("root path is not a directory: %s", rootDir)
	}

	// Test if we can actually read the directory
	_, err = os.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("cannot read root directory %s: %v", rootDir, err)
	}

	return nil
}

func isMountPoint(path string) bool {
	// Check if path is a mount point by comparing device IDs
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	parent := filepath.Dir(path)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return false
	}

	// If device IDs differ, it's likely a mount point
	stat := info.Sys().(*syscall.Stat_t)
	parentStat := parentInfo.Sys().(*syscall.Stat_t)

	return stat.Dev != parentStat.Dev
}

func checkMountPointHealth(path string) error {
	// Try to read the directory to ensure the mount is healthy
	_, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("mount point unhealthy: %v", err)
	}
	return nil
}

func (s *Server) isPathSafe(requestPath string) bool {
	cleanPath := filepath.Clean(requestPath)
	fullPath := filepath.Join(s.rootDir, cleanPath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}

	return strings.HasPrefix(absPath, s.rootDir)
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Add request timeout for external storage operations
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestPath := r.URL.Path
	if !s.isPathSafe(requestPath) {
		log.Printf("Unsafe path access attempt: %s", requestPath)
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(s.rootDir, requestPath)

	// Check if root mount is still healthy before proceeding
	if err := checkMountPointHealth(s.rootDir); err != nil {
		log.Printf("Mount point check failed: %v", err)
		http.Error(w, "Storage temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else if os.IsPermission(err) {
			log.Printf("Permission denied: %s", fullPath)
			http.Error(w, "Access denied", http.StatusForbidden)
		} else {
			log.Printf("Stat error for %s: %v", fullPath, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if info.IsDir() {
		s.handleDirectory(w, r, fullPath, requestPath)
	} else {
		s.handleFile(w, r, fullPath)
	}
}

func (s *Server) handleDirectory(w http.ResponseWriter, r *http.Request, fullPath, requestPath string) {
	// Use context timeout for directory operations
	ctx := r.Context()

	// Channel to handle directory reading with timeout
	type readResult struct {
		entries []os.DirEntry
		err     error
	}

	resultChan := make(chan readResult, 1)
	go func() {
		entries, err := os.ReadDir(fullPath)
		resultChan <- readResult{entries, err}
	}()

	var entries []os.DirEntry
	var err error

	select {
	case result := <-resultChan:
		entries, err = result.entries, result.err
	case <-ctx.Done():
		log.Printf("Directory read timeout for: %s", fullPath)
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
		return
	}

	if err != nil {
		log.Printf("Failed to read directory %s: %v", fullPath, err)
		if os.IsPermission(err) {
			http.Error(w, "Access denied", http.StatusForbidden)
		} else {
			http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		}
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		// Skip hidden files starting with . (optional security measure)
		// if strings.HasPrefix(entry.Name(), ".") {
		// 	continue
		// }

		info, err := entry.Info()
		if err != nil {
			log.Printf("Failed to get info for %s: %v", entry.Name(), err)
			continue
		}

		fileInfo := FileInfo{
			Name:    info.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			SizeStr: formatSize(info.Size()),
			ModStr:  info.ModTime().Format("2006-01-02 15:04:05"),
		}

		if info.IsDir() {
			fileInfo.SizeStr = "-"
		}

		files = append(files, fileInfo)
	}

	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	var parentPath string
	if requestPath != "/" {
		parentPath = filepath.Dir(strings.TrimSuffix(requestPath, "/"))
		if parentPath != "/" {
			parentPath += "/"
		}
	}

	data := PageData{
		Title:       "File Server - " + requestPath,
		CurrentPath: requestPath,
		ParentPath:  parentPath,
		Files:       files,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := s.template.ExecuteTemplate(w, "directory.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	file, err := os.Open(fullPath)
	if err != nil {
		log.Printf("Failed to open file %s: %v", fullPath, err)
		if os.IsPermission(err) {
			http.Error(w, "Access denied", http.StatusForbidden)
		} else {
			http.Error(w, "Failed to open file", http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Printf("Failed to get file info for %s: %v", fullPath, err)
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers for file serving
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	// Prevent directory listing if somehow a directory gets here
	if info.IsDir() {
		http.Error(w, "Cannot serve directory as file", http.StatusBadRequest)
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer for large file downloads
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Starting file server...\n")
	fmt.Printf("Serving directory: %s\n", s.rootDir)
	if isMountPoint(s.rootDir) {
		fmt.Printf("✓ Detected mount point at: %s\n", s.rootDir)
	}
	fmt.Printf("Listening on: http://localhost:%d\n", s.port)

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func main() {
	var (
		rootDir = flag.String("root", ".", "Root directory to serve")
		port    = flag.Int("port", 8080, "Port to listen on")
		help    = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		fmt.Println("Simple Web File Server")
		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./fileserver -root /var/www -port 8080")
		fmt.Println("  ./fileserver -root /home/user/documents")
		fmt.Println("  ./fileserver -root /mnt/external-drive")
		return
	}

	server, err := NewServer(*rootDir, *port)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Shutting down gracefully...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		} else {
			fmt.Println("Server stopped gracefully")
		}

	case err := <-serverErr:
		log.Fatal("Server error:", err)
	}
}
