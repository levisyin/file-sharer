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

type FileInfo struct {
	Name string
}

var (
	//go:embed index.html
	homePage embed.FS
)

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/uploadFile", uploadFile)
	http.ListenAndServe(":8080", nil)
}

func home(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFS(homePage, "index.html")
	if err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("index.html not found in pages cache")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	dir, err := os.Getwd()
	if err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("Getwd error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("ReadDir error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		fileInfos = append(fileInfos, FileInfo{Name: file.Name()})
	}
	data := struct{ Files []FileInfo }{fileInfos}
	if err := tpl.Execute(w, data); err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("Execute tpl error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	// upload size
	err := r.ParseMultipartForm(1024000) // grab the multipart form
	if err != nil {
		slog.With("err", err).Error("ParseMultipartForm error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		out, err := os.Create(fileHeader.Filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	fmt.Fprintf(w, `{"code":0,"msg":"Upload file success!"}`)
}
