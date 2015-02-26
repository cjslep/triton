package triton

import (
	"bufio"
	"errors"
	"gopkg.in/fsnotify.v1"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	WebHost         *http.Server
	ErrChan         chan error
	staticTemplates map[string]string
	staticAssets    map[string][]byte
	templates       *template.Template
}

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
	tmplFiles, ok := cw.Files(".tmpl")
	if !ok {
		return errors.New("No content walker .tmpl files")
	}
	s.templates, err = template.ParseFiles(tmplFiles...)
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
		if filepath.Ext(file) != "" {
			base = baseWithExt[:strings.LastIndex(baseWithExt, ".")]
		}
		s.staticTemplates[base] = "/" + rel
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
	var r RecursiveWatcher
	w, err := r.RecursivelyWatch(pwd)
	if err != nil {
		s.ErrChan <- err
		close(s.ErrChan)
		return
	}
	for {
		select {
		case err = <-w.Errors:
			s.ErrChan <- err
		case event := <-w.Events:
			switch event.Op {
			case fsnotify.Write: // Only for files: reparse
				fallthrough
			case fsnotify.Rename:
				fallthrough
			case fsnotify.Create:
				fallthrough
			case fsnotify.Remove:
				s.initializeContent()
				s.applyHandlers()
			}
		}
	}
}

func (s *Server) ListenAndServe() error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	return s.WebHost.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile string, keyFile string) error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	return s.WebHost.ListenAndServeTLS(certFile, keyFile)
}
