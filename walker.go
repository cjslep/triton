package triton

import (
	"os"
	"path/filepath"
	"strings"
)

// ContentWalker aggregates a list of all files matching any arbitrary file
// extensions. Files are marked hidden if they are within a directory that
// begins with a "." (a dot-directory).
type contentWalker struct {
	gitDirs     map[string]*[]string
	files       map[string]*[]string
	hiddenFiles map[string]*[]string
}

// NewContentWalkerGitDirectories creates a new ContentWalker that will index
// the given file extensions and git directories.
func newContentWalkerGitDirectories(gitDirs []string, fileExts ...string) *contentWalker {
	cw := &contentWalker{
		gitDirs:     make(map[string]*[]string),
		files:       make(map[string]*[]string),
		hiddenFiles: make(map[string]*[]string),
	}
	for _, f := range fileExts {
		arr := make([]string, 0)
		cw.files[f] = &arr
		arrHidden := make([]string, 0)
		cw.hiddenFiles[f] = &arrHidden
	}
	for _, r := range gitDirs {
		arr := make([]string, 0)
		cw.gitDirs[r] = &arr
	}
	return cw
}

func newContentWalker(fileExts ...string) *contentWalker {
	return newContentWalkerGitDirectories(nil, fileExts...)
}

// Walk implements the filepath.WalkFunc interface for use in a call by the client
// to filepath.Walk. It will generate a list of all files for each extension type
// given to it during construction.
func (c *contentWalker) Walk(path string, into os.FileInfo, err error) error {
	if err != nil {
		return err
	} else if c.handleGitDirectory(path) {
		return nil
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

// Files returns all files of the given extension that were not
// in any dot-directories. It returns false if it did not index
// the given extension.
func (c *contentWalker) Files(ext string) ([]string, bool) {
	pFiles, ok := c.files[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}

// HiddenFiles returns all files of the given extension that were
// in at least one dot-directory. It returns false if it did not
// index the given extension.
func (c *contentWalker) HiddenFiles(ext string) ([]string, bool) {
	pFiles, ok := c.hiddenFiles[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}

func (c *contentWalker) GitDirectories(ext string) ([]string, bool) {
	pFiles, ok := c.gitDirs[ext]
	if pFiles != nil {
		return *pFiles, ok
	} else {
		return nil, ok
	}
}

// Returns true if the path contains a git directory
func (c *contentWalker) handleGitDirectory(path string) bool {
	dirPath := filepath.Dir(path)
	for dirExt, found := range c.gitDirs {
		if strings.Contains(dirPath, dirExt) {
			dirPath = dirPath[:strings.Index(dirPath, dirExt)+len(dirExt)]
			// Only add once
			alreadyHave := false
			for _, elem := range *found {
				if elem == dirPath {
					alreadyHave = true
				}
			}
			if !alreadyHave {
				*found = append(*found, dirPath)
			}
			return true
		}
	}
	return false
}
