package triton

import (
	"bufio"
	"errors"
	"fmt"
	"gopkg.in/fsnotify.v1"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	rootTemplateFile = "#"
	templateFileExt  = ".tmpl"
)

// Server provides static cached web hosting. It searches the current working directory of
// execution recursively for .tmpl files containing valid html/templates. It also caches assets
// whose file extension matches the extensions specified by the client using triton. It watches
// the file system for changes and updates the static site accordingly without restarting.
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
// Static asset files behave as expected with the caveat that they must not be located in any
// dot-directories.
type Server struct {
	// WebHost is used to handle http requests. Do not call
	// the WebHost's ListenAndServe nor ListenAndServeTLS functions. Call
	// the triton Server's variants instead for proper setup to occur.
	//
	// If only http.Server had a function for setting its http.Handler in
	// addition to its exported field, then an interface could be used here
	// instead.
	WebHost *http.Server
	// ErrChan can be provided by the client, or is created after the
	// server starts listening for requests. The client must listen to
	// the channel for errors, and a closed channel indicates that the server is
	// no longer properly updating its content and must be restarted.
	ErrChan chan error
	// AssetFileExtensions is a map of file extensions to treat as static
	// assets when encountered in non-dot-directories. The values of the
	// map are the MIME type. This can be nil.
	AssetFileExtensionsToMIME map[string]string
	// GitDirectories are directories that are not cached in RAM. The
	// directory is to be read and served as a publicly accessible
	// repository. This should generally be set as:
	//         []string{".git"}
	GitDirectories []string
	// Map between relative URL paths and template names to execute.
	staticTemplates map[string]string
	// Cached static assets.
	staticAssets map[string][]byte
	// Cached static HTML content.
	templates *template.Template
	// Path to git repositories to serve
	gitDirs []string
	// Current working directory
	pwd string
}

// initializeContent maps all content to URI locations.
func (s *Server) initializeContent() error {
	var err error = nil
	s.staticTemplates = make(map[string]string)
	s.staticAssets = make(map[string][]byte)
	s.templates = nil
	s.pwd, err = os.Getwd()
	if err != nil {
		return err
	}
	var cw *contentWalker
	if s.AssetFileExtensionsToMIME == nil || len(s.AssetFileExtensionsToMIME) == 0 {
		cw = newContentWalkerGitDirectories(s.GitDirectories, templateFileExt)
	} else {
		allFiles := make([]string, 0, len(s.AssetFileExtensionsToMIME)+1)
		allFiles = append(allFiles, templateFileExt)
		for k, _ := range s.AssetFileExtensionsToMIME {
			allFiles = append(allFiles, k)
		}
		cw = newContentWalkerGitDirectories(s.GitDirectories, allFiles...)
	}
	if err = filepath.Walk(s.pwd, cw.Walk); err != nil {
		return err
	}
	hiddenTmplFiles, okHidden := cw.HiddenFiles(templateFileExt)
	tmplFiles, ok := cw.Files(templateFileExt)
	if !ok {
		return errors.New(fmt.Sprintf("No content walker %s files", templateFileExt))
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
		rel, err := filepath.Rel(s.pwd, file)
		if err != nil {
			return err
		}
		relNoExt := rel
		if filepath.Ext(file) != "" {
			relNoExt = rel[:strings.LastIndex(rel, ".")]
			baseWithExt := filepath.Base(file)
			if baseWithExt[:strings.LastIndex(baseWithExt, ".")] == rootTemplateFile {
				relNoExt = relNoExt[:len(relNoExt)-1]
			}
		}
		s.staticTemplates["/"+relNoExt] = "/" + relNoExt
	}
	if s.AssetFileExtensionsToMIME != nil && len(s.AssetFileExtensionsToMIME) > 0 {
		for assetExt, _ := range s.AssetFileExtensionsToMIME {
			assetFiles, _ := cw.Files(assetExt)
			for _, file := range assetFiles {
				err = s.readStaticAssetsFile(s.pwd, file)
				if err != nil {
					return err
				}
			}
		}
	}
	for _, rawDirExt := range s.GitDirectories {
		if dirs, ok := cw.GitDirectories(rawDirExt); ok {
			for _, dir := range dirs {
				rel, err := filepath.Rel(s.pwd, dir)
				if err != nil {
					return err
				}
				s.gitDirs = append(s.gitDirs, "/"+rel+"/")
			}
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
	buffer := make([]byte, 1024)
	r := bufio.NewReader(file)
	for {
		n, err := r.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		asset = append(asset, buffer[0:n]...)
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
		extToMatch := filepath.Ext(path)
		resultMime := ""
		for ext, mimeType := range s.AssetFileExtensionsToMIME {
			if ext == extToMatch {
				resultMime = mimeType
				break
			}
		}
		basicMux.HandleFunc(path, func(myContent []byte, myMimeType string) func(http.ResponseWriter, *http.Request) {
			return func(wr http.ResponseWriter, req *http.Request) {
				// TODO: range-content
				wr.Header().Add("Content-Type", myMimeType)
				wr.Write(myContent)
			}
		}(content, resultMime))
	}
	for tmplName, path := range s.staticTemplates {
		basicMux.HandleFunc(path, func(myTmplName string, myTemplate *template.Template) func(http.ResponseWriter, *http.Request) {
			return func(wr http.ResponseWriter, req *http.Request) {
				myTemplate.ExecuteTemplate(wr, myTmplName, nil)
			}
		}(tmplName, s.templates))
	}
	for _, gitDir := range s.gitDirs {
		basicMux.HandleFunc(gitDir, func(gitDir string) func(http.ResponseWriter, *http.Request) {
			return func(wr http.ResponseWriter, req *http.Request) {
				serveGitRequest(wr, req, s.pwd+req.RequestURI)
			}
		}(gitDir))
	}
	s.WebHost.Handler = basicMux
}

// async_fsnotifylistener is a separate goroutine handling any changes to the filesystem,
// updating the site without taking it down in the process.
func (s *Server) async_fsnotifylistener() {
	if s.ErrChan == nil {
		s.ErrChan = make(chan error)
	}
	w, err := RecursivelyWatch(s.pwd, s.GitDirectories)
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
				w, err = RecursivelyWatch(s.pwd, s.GitDirectories)
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
	// Apply redirect handler
	redirectMux := http.NewServeMux()
	redirectMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
	})
	httpRedirectServer := &http.Server{Handler: redirectMux}
	go s.async_fsnotifylistener()
	go httpRedirectServer.ListenAndServe()
	return s.WebHost.ListenAndServeTLS(certFile, keyFile)
}
