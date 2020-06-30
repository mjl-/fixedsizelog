package fixedsizelog

import (
	"os"
	"testing"
	"time"
)

func TestFixedsizelog(t *testing.T) {
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%v", err)
		}
	}

	path := "testdata/log.txt"
	pathA := path + ".A"
	pathB := path + ".B"
	os.RemoveAll("testdata")
	err := os.Mkdir("testdata", 0777)
	check(err)

	const maxSize = 20

	w, err := New(path, maxSize)
	check(err)
	_, err = os.Stat(pathA)
	check(err)
	_, err = os.Stat(pathB)
	if err == nil {
		t.Fatalf("New unexpectedly created pathB")
	}

	b1 := []byte("012345\n")
	b1size := int64(len(b1))
	n, err := w.Write(b1)
	check(err)
	if n != len(b1) {
		t.Fatalf("bad write, got %d, expect %d", n, len(b1))
	}
	n, err = w.Write(b1)
	check(err)
	if n != len(b1) {
		t.Fatalf("bad write, got %d, expect %d", n, len(b1))
	}
	fi, err := os.Stat(pathA)
	check(err)
	if fi.Size() != 2*b1size {
		t.Fatalf("unexpected size, got %d, expected %d", fi.Size(), 2*len(b1))
	}

	_, err = os.Stat(pathB)
	if err == nil {
		t.Fatalf("pathB was created unexpectedly")
	}

	// First write for path B.
	n, err = w.Write(b1)
	check(err)
	if n != len(b1) {
		t.Fatalf("bad write, got %d, expect %d", n, len(b1))
	}
	fi, err = os.Stat(pathB)
	check(err)
	if fi.Size() != b1size {
		t.Fatalf("unexpected size, got %d, expected %d", fi.Size(), len(b1))
	}

	// Second write for path B.
	_, err = w.Write(b1)
	check(err)

	// Write should go to path A again.
	_, err = w.Write(b1)
	check(err)

	// Check that path A was truncated and is smaller than it was.
	fi, err = os.Stat(pathA)
	check(err)
	if fi.Size() != b1size {
		t.Fatalf("unexpected size, got %d, expected %d", fi.Size(), len(b1))
	}

	// Need to sleep for file systems with 1 second resolution mtimes...
	time.Sleep(1 * time.Second)

	// Second write to path A.
	_, err = w.Write(b1)
	check(err)

	err = w.Close()
	check(err)

	// Reopen file. We wrote to A last, so that should be opened. But on write it'll turn out the file is full already and B is written.
	w, err = New(path, maxSize)
	check(err)

	ww := w.(*writer)
	if ww.index != 0 {
		t.Fatalf("index should be at 0, is %d", ww.index)
	}
	_, err = w.Write(b1)
	check(err)
	if ww.index != 1 {
		t.Fatalf("index should be at 1, is %d", ww.index)
	}
}
