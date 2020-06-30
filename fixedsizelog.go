/*
Package fixedsizelog makes it easy to write logging to files with a preconfigured maximum size.

A preconfigured maximum size prevents logging from filling the disk.

The approach taken by fixedsizelog is very simple: It starts writing logging to
an "A" file. When it is half the total maximum size, it switches to a "B" file,
first truncating it. When the "B" file is full it switches to "A" again. This
means you'll have at least logging history of half of the preconfigured maximum
size.
*/
package fixedsizelog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var errClosed = errors.New("log closed")

type writer struct {
	sync.Mutex
	maxHalfSize int64
	paths       []string

	// current active file
	index int
	file  *os.File
	size  int64
}

func (w *writer) Write(buf []byte) (int, error) {
	w.Lock()
	defer w.Unlock()

	if w.file == nil {
		return -1, errClosed
	}
	if w.size >= w.maxHalfSize {
		if err := w.toOther(); err != nil {
			return -1, err
		}
	}
	n, err := w.file.Write(buf)
	if n > 0 {
		w.size += int64(n)
	}
	return n, err
}

func (w *writer) Close() error {
	w.Lock()
	defer w.Unlock()

	if w.file == nil {
		return errClosed
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *writer) toOther() error {
	f, err := os.OpenFile(w.paths[1-w.index], os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	err = w.file.Close()
	w.file = f
	w.size = 0
	w.index = 1 - w.index
	return err
}

// New creates a new fixedsizelog with files names path + ".A" and path + ".B",
// where both combined have a maximum size of maxSize bytes.
//
// The returned writer is safe for concurrent access.
func New(path string, maxSize int64) (io.WriteCloser, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("maxSize should be >0")
	}

	paths := []string{path + ".A", path + ".B"}

	w := &writer{maxHalfSize: maxSize / 2, paths: paths}

	// Open the file with the most recent mtime.
	// Deal with files that don't exist, without creating unnecessarily.
	f0, err0 := os.OpenFile(paths[0], os.O_WRONLY|os.O_APPEND, 0666)
	f1, err1 := os.OpenFile(paths[1], os.O_WRONLY|os.O_APPEND, 0666)
	defer func() {
		if f0 != nil {
			f0.Close()
		}
		if f1 != nil {
			f1.Close()
		}
	}()
	if err0 != nil || err1 != nil {
		if err0 != nil && !os.IsNotExist(err0) {
			return nil, err0
		}
		if err1 != nil && !os.IsNotExist(err1) {
			return nil, err1
		}
		if err0 == nil {
			w.index = 0
			w.file = f0
			fi, err := f0.Stat()
			if err != nil {
				return nil, err
			}
			w.size = fi.Size()
			f0 = nil
			return w, nil
		}
		if err1 == nil {
			w.index = 1
			w.file = f1
			fi, err := f1.Stat()
			if err != nil {
				return nil, err
			}
			w.size = fi.Size()
			f1 = nil
			return w, nil
		}

		f, err := os.OpenFile(w.paths[0], os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		w.file = f
		return w, nil
	}

	fi0, err0 := f0.Stat()
	fi1, err1 := f1.Stat()
	if err0 != nil {
		return nil, err0
	}
	if err1 != nil {
		return nil, err1
	}
	if fi0.ModTime().After(fi1.ModTime()) {
		w.index = 0
		w.file = f0
		w.size = fi0.Size()
		f0 = nil
	} else {
		w.index = 1
		w.file = f1
		w.size = fi1.Size()
		f1 = nil
	}

	return w, nil
}
