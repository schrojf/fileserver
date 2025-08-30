package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
}

type Server struct {
	rootDir  string
	port     int
	template *template.Template
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

	return &Server{
		rootDir:  absRoot,
		port:     port,
		template: tmpl,
	}, nil
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
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestPath := r.URL.Path
	if !s.isPathSafe(requestPath) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(s.rootDir, requestPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not found", http.StatusNotFound)
		} else {
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
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
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

	parentPath := ""
	if requestPath != "/" {
		parentPath = filepath.Dir(filepath.Join(requestPath, ".."))
		if parentPath != "/" {
			parentPath = parentPath + "/"
		}
	}

	data := PageData{
		Title:       "File Server - " + requestPath,
		CurrentPath: requestPath,
		ParentPath:  parentPath,
		Files:       files,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.template.ExecuteTemplate(w, "directory.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	file, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
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
		return
	}

	server, err := NewServer(*rootDir, *port)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	fmt.Printf("Starting file server...\n")
	fmt.Printf("Serving directory: %s\n", server.rootDir)
	fmt.Printf("Listening on: http://localhost:%d\n", *port)

	http.HandleFunc("/", server.handleRequest)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
