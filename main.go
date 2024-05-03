package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/pflag"
)

type FileInfo struct {
	Name string
}

var (
	//go:embed index.html
	homePage embed.FS
)

var (
	addr = pflag.String("addr", ":8080", "Listen addr")
	root = pflag.String("path", "./", "The root path to serve")
)

var (
	serveRoot = ""
)

func main() {
	figure.NewFigure("File Sharer", "standard", true).Print()
	fmt.Println()
	pflag.Parse()
	if filepath.IsAbs(*root) {
		serveRoot = *root
	} else {
		pwd, err := os.Getwd()
		if err != nil {
			slog.With("err", err).Error("get current path error")
			return
		}
		serveRoot = filepath.Join(pwd, *root)
	}
	http.HandleFunc("/", home)
	http.HandleFunc("/uploadFile", uploadFile)
	slog.Info(fmt.Sprintf("File sharer listening on %s", *addr))
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		slog.With("err", err).Error("start file sharer error")
	}
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

	files, err := os.ReadDir(serveRoot)
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
	err := r.ParseMultipartForm(1024000)
	if err != nil {
		slog.With("err", err).Error("ParseMultipartForm error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			slog.With("err", err).Error("Open file error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		out, err := os.Create(filepath.Join(serveRoot, fileHeader.Filename))
		if err != nil {
			slog.With("err", err).Error("Create file error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			slog.With("err", err).Error("Copy file error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slog.With("f", fileHeader.Filename).Info("upload file success")
	}
	fmt.Fprintf(w, `{"code":0,"msg":"Upload file success!"}`)
}
