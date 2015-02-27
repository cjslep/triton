package triton

import (
	"os"
	"path/filepath"
	"strings"
)

type ContentWalker struct {
	files       map[string]*[]string
	hiddenFiles map[string]*[]string
}

func NewContentWalker(fileExts ...string) *ContentWalker {
	cw := &ContentWalker{
		files:       make(map[string]*[]string),
		hiddenFiles: make(map[string]*[]string),
	}
	for _, f := range fileExts {
		arr := make([]string, 0)
		cw.files[f] = &arr
		arrHidden := make([]string, 0)
		cw.hiddenFiles[f] = &arrHidden
	}
	return cw
}

func (c *ContentWalker) Walk(path string, into os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	ext := filepath.Ext(path)
	toSearch := c.files
	if strings.Contains(filepath.Dir(path), "/.") {
		toSearch = c.hiddenFiles
	}
	for k, v := range toSearch {
		if k == ext && filepath.Base(path)[0] != '.' {
			*v = append(*v, path)
			break
		}
	}
	return nil
}

func (c *ContentWalker) Files(ext string) ([]string, bool) {
	pFiles, ok := c.files[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}

func (c *ContentWalker) HiddenFiles(ext string) ([]string, bool) {
	pFiles, ok := c.hiddenFiles[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}
