package triton

import (
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
)

// RecursiveWatcher is a list of subdirectories under a user-supplied root, to allow
// easy creation of a recursive *fsnotify.Watcher.
type recursiveWatcher []string

// RecursivelyWatch returns a *fsnotify.Watcher that is watching all subdirectories
// under the given path root. The watcher is invalid if a non-nil error is returned.
func (r *recursiveWatcher) RecursivelyWatch(root string) (*fsnotify.Watcher, error) {
	*r = recursiveWatcher(make([]string, 0))
	err := filepath.Walk(root, r.walk)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, path := range *r {
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
	if info.IsDir() {
		*r = append(*r, path)
	}
	return nil
}
