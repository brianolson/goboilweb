package main

import (
	"database/sql"
	"embed"
	"flag"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed templates
var tfs embed.FS

//go:embed static
var sfs embed.FS

var templates *template.Template
var db *sql.DB

var prodMode *bool = flag.Bool("prod", false, "use minimized js, turn off debugging, etc")
var sqlDriver *string = flag.String("sql-driver", "postgres", "name of go sql driver to connect to db with")
var sqlConnectionString *string = flag.String("sql-db", "", "connection parameter strig for sql db")

func dbConnect() (*sql.DB, error) {
	if len(*sqlConnectionString) == 0 {
		log.Print("no --sql-db config, not opening")
		return nil, nil
	}
	db, err := sql.Open(*sqlDriver, *sqlConnectionString)
	return db, err
}

func loadTemplates() error {
	nt, err := template.ParseFS(tfs, "templates/*.html")
	if err != nil {
		log.Print("error listing template html ", err)
		return err
	}
	templates = nt
	return err
}

func reloadTemplates() {
	if *prodMode {
		return
	}
	err := loadTemplates()
	if err != nil {
		log.Print("err reloading templates", err)
	}
}

func baseHandler(out http.ResponseWriter, request *http.Request) {
	log.Print("baseHandler " + request.URL.Path)
	var err error
	if !*prodMode {
		reloadTemplates()
	}
	t := templates.Lookup("index.html")
	if t == nil {
		log.Fatal("no base.html template!")
		http.Error(out, "template error base.html", 500)
		return
	}
	context := struct {
		DateStr string
	}{
		time.Now().Format(time.RFC3339Nano),
	}
	out.Header()["Content-Type"] = []string{"text/html"}
	err = t.Execute(out, context)
	if err != nil {
		log.Print(err)
	}
}

func faviconHandler(out http.ResponseWriter, request *http.Request) {
	faviconBytes, err := sfs.ReadFile("static/favicon.ico")
	if err != nil {
		http.NotFound(out, request)
		return
	}
	out.Header()["Content-Type"] = []string{"image/vnd.microsoft.icon"}
	out.WriteHeader(http.StatusOK)
	out.Write(faviconBytes)
}

type StaticHandler struct {
	stripPrefix string
	newPrefix   string
	fsHandler   http.Handler
}

func (sh *StaticHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, sh.stripPrefix) && ((req.URL.RawPath == "") || strings.HasPrefix(req.URL.RawPath, sh.stripPrefix)) {
		req.URL.Path = strings.Replace(req.URL.Path, sh.stripPrefix, sh.newPrefix, 1)
		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.Replace(req.URL.RawPath, sh.stripPrefix, sh.newPrefix, 1)
		}
		sh.fsHandler.ServeHTTP(rw, req)
	} else {
		log.Printf("Path=%#v RawPath=%#v, prefix=%#v", req.URL.Path, req.URL.RawPath, sh.stripPrefix)
		http.Error(rw, "nope", http.StatusNotFound)
	}
}

func main() {
	var err error
	serveAddr := flag.String("addr", ":8777", "Server Addr")
	flag.Parse()

	err = loadTemplates()
	if err != nil {
		log.Fatal(err)
	}

	db, _ = dbConnect()

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", faviconHandler)
	// In production static file serving is probably handed elsewhere, but in dev, here it is
	sh := StaticHandler{stripPrefix: "/s/", newPrefix: "/static/", fsHandler: http.FileServer(http.FS(sfs))}
	mux.Handle("/s/", &sh)
	mux.HandleFunc("/", baseHandler)

	server := &http.Server{
		Addr:    *serveAddr,
		Handler: mux,
	}
	log.Print("serving on ", *serveAddr)
	log.Fatal(server.ListenAndServe())
}
