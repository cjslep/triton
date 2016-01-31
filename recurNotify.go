package triton

import (
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
	"strings"
)

// RecursiveWatcher is a list of subdirectories under a user-supplied root, to allow
// easy creation of a recursive *fsnotify.Watcher.
type recursiveWatcher struct {
	watch          []string
	ignoreIfInPath []string
}

// RecursivelyWatch returns a *fsnotify.Watcher that is watching all subdirectories
// under the given path root. The watcher is invalid if a non-nil error is returned.
func RecursivelyWatch(root string, ignoreIfInPath []string) (*fsnotify.Watcher, error) {
	r := &recursiveWatcher{
		watch:          make([]string, 0),
		ignoreIfInPath: ignoreIfInPath,
	}
	err := filepath.Walk(root, r.walk)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, path := range r.watch {
		err = watcher.Add(path)
		if err != nil {
			return nil, err
		}
	}
	return watcher, nil
}

// walk implements filepath.WalkFunc and simply creates a list of all directories
// encountered.
func (r *recursiveWatcher) walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() && !r.isIgnored(path) {
		r.watch = append(r.watch, path)
	}
	return nil
}

func (r *recursiveWatcher) isIgnored(path string) bool {
	for _, ignoredPart := range r.ignoreIfInPath {
		if strings.Contains(path, ignoredPart) {
			return true
		}
	}
	return false
}
