package triton

import (
	"os"
	"path/filepath"
)

type ContentWalker map[string]*[]string

func NewContentWalker(fileExts ...string) *ContentWalker {
	cw := make(map[string]*[]string)
	for _, f := range fileExts {
		cw[f] = nil
	}
	contentW := ContentWalker(cw)
	return &contentW
}

func (c *ContentWalker) Walk(path string, into os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	ext := filepath.Ext(path)
	for k, v := range *c {
		if k == ext {
			*v = append(*v, path)
			break
		}
	}
	return nil
}

func (c *ContentWalker) Files(ext string) ([]string, bool) {
	pFiles, ok := (*c)[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}
