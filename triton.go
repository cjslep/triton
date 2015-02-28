package triton

import (
	"bufio"
	"errors"
	"fmt"
	"gopkg.in/fsnotify.v1"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Server provides static web hosting and a very minimal Content-Management-System, if it
// could even be called that. It searches the current working directory of execution recursively
// for .tmpl files containing valid html/templates, .css files, and .js files. It watches the file
// system for changes and updates the static site accordingly without restarting.
//
// Template files provide two different, but simple, behaviors depending whether they are located
// in dot-directories.
//
// If a .tmpl file is within a dot directory, it is parsed and added to the master static Template
// for use by other templates.
//
// If a .tmpl file is not within a dot directory, it is parsed and added to the master static
// Template. Additionally, a URL entry to that file location and template filename (without the
// extension) is created for web clients to visit. The template executed to create that particular
// page will be the template filename (again, without extension). A quick example:
//
// If the Server was executed at <ROOT>, hosting at SITE.com, and a template file is located at
// <ROOT>/foo/bar/baz.tmpl, then a web client can go to SITE.com/foo/bar/baz.tmpl and the server
// will execute the template named "baz" and serve it.
//
// The special case is the file "#.tmpl", which will serve and match the directory it is contained
// in. So if <ROOT> hosting SITE.com has <ROOT>/#.tmpl, then the client can go to SITE.com (the
// home page!) and be served template "/".
//
// CSS and Javascript files behave as expected with the caveat that they must not be located in
// any dot-directories.
type Server struct {
	// WebHost is used to handle http requests. Do not call
	// the WebHost's ListenAndServe nor ListenAndServeTLS functions. Call
	// the triton Server's variants instead for proper setup to occur.
	WebHost *http.Server
	// ErrChan can be provided by the client, or is created after the
	// server starts listening for requests. The client must listen to
	// the channel for errors, and a closed channel indicates that the server is
	// no longer properly updating its content and must be restarted.
	ErrChan chan error
	// Map between relative URL paths and template names to execute.
	staticTemplates map[string]string
	// Cached static assets.
	staticAssets map[string][]byte
	// Cached static HTML content.
	templates *template.Template
}

// initializeContent maps all content to URI locations.
func (s *Server) initializeContent() error {
	s.staticTemplates = make(map[string]string)
	s.staticAssets = make(map[string][]byte)
	s.templates = nil
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cw := NewContentWalker(".tmpl", ".css", ".js")
	if err = filepath.Walk(pwd, cw.Walk); err != nil {
		return err
	}
	hiddenTmplFiles, okHidden := cw.HiddenFiles(".tmpl")
	tmplFiles, ok := cw.Files(".tmpl")
	if !ok {
		return errors.New(fmt.Sprintf("No content walker .tmpl files"))
	}
	if okHidden && len(hiddenTmplFiles) > 0 {
		s.templates, err = template.ParseFiles(hiddenTmplFiles...)
		if err != nil {
			return err
		}
		s.templates, err = s.templates.ParseFiles(tmplFiles...)
	} else {
		s.templates, err = template.ParseFiles(tmplFiles...)
	}
	if err != nil {
		return err
	}
	for _, file := range tmplFiles {
		rel, err := filepath.Rel(pwd, file)
		if err != nil {
			return err
		}
		baseWithExt := filepath.Base(file)
		base := baseWithExt
		relNoExt := rel
		if filepath.Ext(file) != "" {
			base = baseWithExt[:strings.LastIndex(baseWithExt, ".")]
			relNoExt = rel[:strings.LastIndex(rel, ".")]
			if base == "#" {
				base = "/" + relNoExt[:len(relNoExt)-1]
				relNoExt = relNoExt[:len(relNoExt)-1]
			}
		}
		s.staticTemplates[base] = "/" + relNoExt
	}
	cssFiles, _ := cw.Files(".css")
	for _, file := range cssFiles {
		err = s.readStaticAssetsFile(pwd, file)
		if err != nil {
			return err
		}
	}
	jsFiles, _ := cw.Files(".js")
	for _, file := range jsFiles {
		err = s.readStaticAssetsFile(pwd, file)
		if err != nil {
			return err
		}
	}
	return nil
}

// readStaticAssetsFile associates a specific asset file path to its content.
func (s *Server) readStaticAssetsFile(baseDir string, assetFile string) error {
	file, err := os.Open(assetFile)
	if err != nil {
		return err
	}
	defer file.Close()
	var asset []byte
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		asset = append(asset, scan.Bytes()...)
	}
	relPath, err := filepath.Rel(baseDir, assetFile)
	if err != nil {
		return err
	}
	s.staticAssets["/"+relPath] = asset
	return nil
}

// applyHandlers takes the URI mapped static content and creates web handlers to service
// the content.
func (s *Server) applyHandlers() {
	basicMux := http.NewServeMux()
	for path, content := range s.staticAssets {
		basicMux.HandleFunc(path, func(myContent []byte) func(http.ResponseWriter, *http.Request) {
			return func(wr http.ResponseWriter, req *http.Request) {
				wr.Write(myContent)
			}
		}(content))
	}
	for tmplName, path := range s.staticTemplates {
		basicMux.HandleFunc(path, func(myTmplName string, myTemplate *template.Template) func(http.ResponseWriter, *http.Request) {
			return func(wr http.ResponseWriter, req *http.Request) {
				myTemplate.ExecuteTemplate(wr, myTmplName, nil)
			}
		}(tmplName, s.templates))
	}
	s.WebHost.Handler = basicMux
}

// async_fsnotifylistener is a separate goroutine handling any changes to the filesystem,
// updating the site without taking it down in the process.
func (s *Server) async_fsnotifylistener() {
	if s.ErrChan == nil {
		s.ErrChan = make(chan error)
	}
	pwd, err := os.Getwd()
	if err != nil {
		s.ErrChan <- err
		close(s.ErrChan)
		return
	}
	tempR := recursiveWatcher(make([]string, 0))
	r := &tempR
	w, err := r.RecursivelyWatch(pwd)
	if err != nil {
		s.ErrChan <- err
		close(s.ErrChan)
		return
	}
	defer w.Close()
	for {
		select {
		case err = <-w.Errors:
			s.ErrChan <- err
		case event := <-w.Events:
			// The following could be quite smarter: for now,
			// be dumb and rebuild the eentire tree.
			switch event.Op {
			case fsnotify.Write:
				fallthrough
			case fsnotify.Rename:
				fallthrough
			case fsnotify.Create:
				fallthrough
			case fsnotify.Remove:
				s.initializeContent()
				s.applyHandlers()
				err = w.Close()
				if err != nil {
					s.ErrChan <- err
					close(s.ErrChan)
					return
				}
				w, err = r.RecursivelyWatch(pwd)
				if err != nil {
					s.ErrChan <- err
					close(s.ErrChan)
					return
				}
			}
		}
	}
}

// ListenAndServe initializes the Server before using the WebHost to serve http
// requests.
func (s *Server) ListenAndServe() error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	go s.async_fsnotifylistener()
	return s.WebHost.ListenAndServe()
}

// ListenAndServeTLS initializes the Server before using the WebHost to serve
// http over TLS.
func (s *Server) ListenAndServeTLS(certFile string, keyFile string) error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	go s.async_fsnotifylistener()
	return s.WebHost.ListenAndServeTLS(certFile, keyFile)
}
