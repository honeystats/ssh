package main

import (
	"fmt"
	"testing"
)

func assertPathEqual(t *testing.T, dir *FilesystemDir, expected string) {
	dirPath := dir.Path()
	if dirPath != expected {
		t.Errorf("Dir named [%s] had path [%s], expected [%s]\n", dir.Name, dirPath, expected)
	}
}

func TestPaths(t *testing.T) {
	assertPathEqual(t, FILESYSTEM.Root, "/")

	_, home := getDir(FILESYSTEM.Root, "/home")
	assertPathEqual(t, home, "/home")
	fmt.Printf("home: %#v\n", home)

	_, ubuntu := getDir(FILESYSTEM.Root, "/home/ubuntu")
	assertPathEqual(t, ubuntu, "/home/ubuntu")
}
