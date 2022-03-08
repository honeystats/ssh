package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/honeystats/ssh/files"
	"github.com/sirupsen/logrus"
)

func realPathToDir(startPath string, firstPass bool) (*files.FilesystemDir, error) {
	root := &files.FilesystemDir{
		Subdirs: []*files.FilesystemDir{},
		Files:   []*files.FilesystemFile{},
	}
	if !firstPass {
		root.Name = filepath.Base(startPath)
	}
	dirFiles, err := ioutil.ReadDir(startPath)
	if err != nil {
		return nil, err
	}
	for _, file := range dirFiles {
		if file.IsDir() {
			path := filepath.Join(startPath, file.Name())
			var res *files.FilesystemDir
			res, err = realPathToDir(path, false)
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
	rootDir, err := realPathToDir(path, true)
	if err != nil {
		return nil, err
	}

	return &files.FilesystemConfig{
		Root: rootDir,
	}, nil
}

var pathToLookup string

func main() {
	flag.StringVar(&pathToLookup, "source-path", "", "path from which to generate the YAML file")
	flag.Parse()
	if pathToLookup == "" {
		logrus.Fatalln("Missing source-path arg.")
	}

	conf, err := realPathToConfig(pathToLookup)
	if err != nil {
		logrus.WithError(err).Fatalln("error getting info for dir")
	}
	str, err := conf.ToString()
	if err != nil {
		logrus.WithError(err).Fatalln("error marshalling dir to string")
	}
	fmt.Println(str)
}
