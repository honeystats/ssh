package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type FileLike interface {
	GetName() string
	GetPermissions() int
}

type FilesystemFile struct {
	Name    string
	Content string
}

func (f FilesystemFile) GetName() string {
	return f.Name
}

func (f FilesystemFile) GetPermissions() int {
	return 0
}

type FilesystemDir struct {
	Name        string
	Permissions int
	Subdirs     []FilesystemDir
	Files       []FilesystemFile
}

func (d FilesystemDir) GetSubdir(name string) (error, FilesystemDir) {
	if name == "" {
		return nil, d
	}
	for _, subdir := range d.Subdirs {
		if subdir.Name == name {
			return nil, subdir
		}
	}
	return errors.New("No subdir with name " + name), FilesystemDir{}
}

func (d FilesystemDir) GetFile(name string) (error, FilesystemFile) {
	for _, file := range d.Files {
		if file.Name == name {
			return nil, file
		}
	}
	return errors.New("No file with name " + name), FilesystemFile{}
}

func (d FilesystemDir) GetName() string {
	return d.Name
}

func (d FilesystemDir) GetPermissions() int {
	return d.Permissions
}

type FilesystemConfig struct {
	Root FilesystemDir
}

func strToFilesystem(cfg []byte) FilesystemConfig {
	ret := FilesystemConfig{}
	yaml.Unmarshal(cfg, &ret)
	return ret
}

var FILESYSTEM FilesystemConfig
var CURRENT_DIR = "/"

func getFileAtPath(path string) FileLike {
	return FilesystemFile{}
}

func init() {
	filesConfig, configSet := os.LookupEnv("FILES_CONFIG")
	if !configSet {
		panic("FILES_CONFIG is not set.")
	}

	bytes, err := ioutil.ReadFile(filesConfig)
	if err != nil {
		log.Panic(err)
	}
	FILESYSTEM = strToFilesystem(bytes)
}

func DescribeFileLike(f FileLike) string {
	return fmt.Sprintf("NAME: [%s] PERMS: [%d]\n", f.GetName(), f.GetPermissions())
}

func getFileOrDir(path string) (error, FileLike) {
	pathParts := strings.Split(path, "/")
	var currentDir FilesystemDir
	currentDir = FILESYSTEM.Root
	if path != "/" {
		for i, part := range pathParts {
			if i == 0 {
				continue
			}
			err, newDir := currentDir.GetSubdir(part)
			if err == nil {
				currentDir = newDir
			} else {
				if i == len(pathParts)-1 {
					fileErr, file := currentDir.GetFile(part)
					if fileErr != nil {
						return fileErr, nil
					}
					return nil, file
				} else {
					return err, nil
				}
			}
		}
	}
	return nil, currentDir
}

func ls(path string) (error, string) {
	err, f := getFileOrDir(path)
	if err != nil {
		return err, ""
	}
	return nil, DescribeFileLike(f)
}
