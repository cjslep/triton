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
	hiddenTmplFiles, okHidden := cw.HiddenFiles(".tmpl")
	tmplFiles, ok := cw.Files(".tmpl")
	if !ok {
		return errors.New(fmt.Sprintf("No content walker .tmpl files"))
	}
	if okHidden {
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
	tempR := RecursiveWatcher(make([]string, 0))
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

func (s *Server) ListenAndServe() error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	go s.async_fsnotifylistener()
	return s.WebHost.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile string, keyFile string) error {
	err := s.initializeContent()
	if err != nil {
		return err
	}
	s.applyHandlers()
	go s.async_fsnotifylistener()
	return s.WebHost.ListenAndServeTLS(certFile, keyFile)
}
