package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/honeystats/ssh/files"
	"github.com/sirupsen/logrus"
)

func realPathToDir(startPath string) (*files.FilesystemDir, error) {
	rootName := filepath.Base(startPath)
	root := &files.FilesystemDir{
		Name:    rootName,
		Subdirs: []*files.FilesystemDir{},
		Files:   []*files.FilesystemFile{},
	}
	dirFiles, err := ioutil.ReadDir(startPath)
	if err != nil {
		return nil, err
	}
	for _, file := range dirFiles {
		if file.IsDir() {
			path := filepath.Join(startPath, file.Name())
			var res *files.FilesystemDir
			res, err = realPathToDir(path)
			if err != nil {
				return nil, err
			}
			root.Subdirs = append(root.Subdirs, res)
		} else {
			filePath := filepath.Join(startPath, file.Name())
			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			newFile := &files.FilesystemFile{
				Name:    file.Name(),
				Content: string(content),
			}
			root.Files = append(root.Files, newFile)
		}
	}
	return root, nil
}

func realPathToConfig(path string) (*files.FilesystemConfig, error) {
	rootDir, err := realPathToDir(path)
	if err != nil {
		return nil, err
	}

	return &files.FilesystemConfig{
		Root: rootDir,
	}, nil
}

func main() {
	conf, err := realPathToConfig("/home/kyle/Downloads/example")
	if err != nil {
		logrus.WithError(err).Fatalln("error getting info for dir")
	}
	str, err := conf.ToString()
	if err != nil {
		logrus.WithError(err).Fatalln("error marshalling dir to string")
	}
	fmt.Println(str)
}
