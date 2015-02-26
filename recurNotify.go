package triton

import (
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
)

type RecursiveWatcher []string

func (r *RecursiveWatcher) RecursivelyWatch(root string) (*fsnotify.Watcher, error) {
	*r = RecursiveWatcher(make([]string, 0))
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

func (r *RecursiveWatcher) walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		*r = append(*r, path)
	}
	return nil
}
