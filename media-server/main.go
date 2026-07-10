package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	addr := flag.String("addr", envOrDefault("LISTEN_ADDR", ":80"), "HTTP listen address")
	root := flag.String("root", envOrDefault("MEDIA_ROOT", "."), "directory to serve")
	flag.Parse()

	info, err := os.Stat(*root)
	if err != nil {
		log.Fatalf("stat root directory: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("root must be a directory: %s", *root)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", fileHandler(http.Dir(*root)))

	server := &http.Server{
		Addr:    *addr,
		Handler: logRequests(mux),
	}

	log.Printf("serving %s on %s", *root, displayURL(*addr))
	log.Fatal(server.ListenAndServe())
}

func fileHandler(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name, ok := cleanPath(r.URL.Path)
		if !ok {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}

		file, err := fs.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			http.Error(w, "stat file", http.StatusInternalServerError)
			return
		}
		if stat.IsDir() {
			if !strings.HasSuffix(r.URL.Path, "/") {
				http.Redirect(w, r, path.Base(r.URL.Path)+"/", http.StatusMovedPermanently)
				return
			}
			listDirectory(w, r, file)
			return
		}

		setFileHeaders(w, stat.Name())
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	})
}

func setFileHeaders(w http.ResponseWriter, filename string) {
	w.Header().Set("Accept-Ranges", "bytes")

	contentType := mediaContentType(filename)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	if isInlineMedia(filename) {
		w.Header().Set("Content-Disposition", "inline")
	}
}

func mediaContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return mime.TypeByExtension(ext)
}

func isInlineMedia(filename string) bool {
	contentType := mediaContentType(filename)
	return strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/")
}

type dirEntry struct {
	Name          string
	URL           string
	Size          int64
	SizeFormatted string
	Dir           bool
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %c", float64(bytes)/float64(div), "KMGTPE"[exp])
}

type dirPage struct {
	Path    string
	Parent  string
	Entries []dirEntry
}

var dirTemplate = template.Must(template.New("dir").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Index of {{.Path}}</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 2rem; line-height: 1.45; }
    h1 { font-size: 1.4rem; margin-bottom: 1rem; }
    table { border-collapse: collapse; width: 100%; max-width: 960px; }
    th, td { border-bottom: 1px solid #ddd; padding: .45rem .35rem; text-align: left; }
    th:last-child, td:last-child { text-align: right; }
    a { color: #0645ad; text-decoration: none; }
    a:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <h1>Index of {{.Path}}</h1>
  <table>
    <thead><tr><th>Name</th><th>Size</th></tr></thead>
    <tbody>
      {{if .Parent}}<tr><td><a href="{{.Parent}}">../</a></td><td>-</td></tr>{{end}}
      {{range .Entries}}
      <tr><td><a href="{{.URL}}">{{.Name}}</a></td><td>{{if .Dir}}-{{else}}{{.SizeFormatted}}{{end}}</td></tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
`))

func listDirectory(w http.ResponseWriter, r *http.Request, file http.File) {
	entries, err := file.Readdir(-1)
	if err != nil {
		http.Error(w, "read directory", http.StatusInternalServerError)
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	page := dirPage{
		Path:    r.URL.Path,
		Entries: make([]dirEntry, 0, len(entries)),
	}
	if r.URL.Path != "/" {
		page.Parent = "../"
	}

	for _, entry := range entries {
		name := entry.Name()
		link := url.PathEscape(name)
		if entry.IsDir() {
			name += "/"
			link += "/"
		}

		page.Entries = append(page.Entries, dirEntry{
			Name:          name,
			URL:           link,
			Size:          entry.Size(),
			SizeFormatted: formatSize(entry.Size()),
			Dir:           entry.IsDir(),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	if err := dirTemplate.Execute(w, page); err != nil {
		log.Printf("render directory listing: %v", err)
	}
}

func cleanPath(rawPath string) (string, bool) {
	if rawPath == "" {
		return "", false
	}

	clean := path.Clean("/" + rawPath)
	if strings.Contains(clean, "\x00") {
		return "", false
	}
	if clean == "/" {
		return ".", true
	}

	return strings.TrimPrefix(clean, "/"), true
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s range=%q", r.Method, r.URL.Path, r.Header.Get("Range"))
		next.ServeHTTP(w, r)
	})
}

func displayURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://0.0.0.0" + addr
	}
	return "http://" + addr
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}
