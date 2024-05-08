package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
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
	//go:embed index.gohtml
	homePage embed.FS
	//go:embed favicon.ico
	favicon []byte
)

var (
	port         = pflag.IntP("port", "p", 8080, "Listen addr")
	root         = pflag.String("root", "./", "The root path to serve")
	onlyLocal    = pflag.Bool("local", false, "Only listening on local machine")
	onlyIntranet = pflag.Bool("intranet", false, "Only listening on intranet")
	advance      = pflag.Bool("advance", false, "Use advance mode")
)

var (
	serveRoot = ""
)

func main() {
	var err error
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
	var listenAddrs string
	switch {
	case *onlyLocal:
		listenAddrs = fmt.Sprintf("127.0.0.1:%d", *port)
		fmt.Printf("listening on: http://127.0.0.1:%d\n", *port)
	case *onlyIntranet:
		localIP := GetLocalIP()
		listenAddrs = fmt.Sprintf("%s:%d", localIP, *port)
		fmt.Printf("listening on: http://%s:%d\n", localIP, *port)
	default:
		interfaces, err := net.Interfaces()
		if err != nil {
			panic(fmt.Errorf("listing net interfaces: %w", err))
		}
		for _, i := range interfaces {
			var addrs []net.Addr
			addrs, err = i.Addrs()
			if err != nil {
				panic(fmt.Errorf("i.Addrs: %w", err))
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				fmt.Printf("listening on: http://%s:%d\n", ip, *port)
			}
		}
		listenAddrs = fmt.Sprintf(":%d", *port)
	}
	if *advance {
		http.HandleFunc("/", home)
		http.HandleFunc("/favicon.ico", ico)
		http.HandleFunc("/uploadFile", uploadFile)
		http.HandleFunc("/downloadFile", downloadFile)
		if err = http.ListenAndServe(listenAddrs, LoggerMiddleware(http.DefaultServeMux)); err != nil {
			slog.With("err", err).Error("start file sharer error")
			return
		}
	} else {
		if err = http.ListenAndServe(listenAddrs, http.FileServer(http.Dir(*root))); err != nil {
			slog.With("err", err).Error("start file sharer error")
			return
		}
	}
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		slog.Info(fmt.Sprintf("%s %s", r.Method, r.RequestURI))
	})
}

func ico(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "favicon.ico")
}

func home(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFS(homePage, "index.gohtml")
	if err != nil {
		slog.With("err", err, "uri", r.RequestURI).Error("index page not found in pages cache")
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

func downloadFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fileName := r.URL.Query().Get("file")
	filePath := filepath.Join(serveRoot, fileName)

	_, err := os.Stat(filePath)
	if err != nil {
		slog.With("err", err).Error("Open file error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	http.ServeFile(w, r, filePath)
}
