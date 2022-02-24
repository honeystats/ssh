package main

import (
	"testing"
)

func assertPathEqual(t *testing.T, fd FileDir, expected string) {
	dirPath := fd.Path()
	if dirPath != expected {
		t.Errorf("Got path [%s], expected [%s]\n", dirPath, expected)
	}
}

func TestPaths(t *testing.T) {
	assertPathEqual(t, FILESYSTEM.Root, "/")

	err, home := FILESYSTEM.Root.getFileOrDir("/home")
	if err != nil {
		t.Error(err)
		return
	}
	assertPathEqual(t, home, "/home")

	_, ubuntu := FILESYSTEM.Root.getFileOrDir("/home/ubuntu")
	assertPathEqual(t, ubuntu, "/home/ubuntu")
}
