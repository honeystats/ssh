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

	err, home := getFileOrDir(FILESYSTEM.Root, "/home")
	if err != nil {
		t.Error(err)
		return
	}
	assertPathEqual(t, home, "/home")

	_, ubuntu := getFileOrDir(FILESYSTEM.Root, "/home/ubuntu")
	assertPathEqual(t, ubuntu, "/home/ubuntu")
}
