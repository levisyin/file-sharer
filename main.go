package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
)

var (
	//go:embed index.html
	homePage embed.FS
)

func main() {
	http.HandleFunc("/", home)
	http.FileServer(http.FS(homePage))
	http.HandleFunc("/uploadFile", uploadFile)
	http.ListenAndServe(":8080", nil)
}

func home(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFS(homePage)
	if err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("index.html not found in pages cache")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	data := map[string]interface{}{
		"userAgent": r.UserAgent(),
	}
	if err := tpl.Execute(w, data); err != nil {
		return
	}
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	// upload size
	err := r.ParseMultipartForm(1024000) // grab the multipart form
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	//reading original file
	file, handler, err := r.FormFile("originalFile")
	if err != nil {
		slog.With("err", err).Error("retrieving file info error")
		return
	}
	defer file.Close()

	resFile, err := os.Create(handler.Filename)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer resFile.Close()
	io.Copy(resFile, file)
	fmt.Fprintf(w, "Upload file success\n")
}
