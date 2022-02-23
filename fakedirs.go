package main

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

type Describable interface {
	Describe() string
	DescribeSelf() string
	PlainName() string
}

type FilesystemFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (f FilesystemFile) PlainName() string {
	return f.Name
}

func (f FilesystemFile) Describe() string {
	return f.DescribeSelf()
}

func (f FilesystemFile) DescribeSelf() string {
	return f.Name
}

type FilesystemDir struct {
	Name        string            `json:"name"`
	Permissions int               `json:"permissions"`
	Subdirs     []*FilesystemDir  `json:"subdirs"`
	Files       []*FilesystemFile `json:"files"`
	Parent      *FilesystemDir    `json:"-"`
}

func (d FilesystemDir) Path() string {
	return d.PathHelp(false)
}

func (d FilesystemDir) PathHelp(belowRoot bool) string {
	parent := d.Parent
	if parent == nil {
		if belowRoot {
			return ""
		} else {
			return "/"
		}
	}

	return parent.PathHelp(true) + "/" + d.Name
}

func (d *FilesystemDir) GetSubdir(name string) (error, *FilesystemDir) {
	if name == "" {
		return nil, d
	}
	for _, subdir := range d.Subdirs {
		if subdir.Name == name {
			return nil, subdir
		}
	}
	return errors.New("No subdir with name " + name), nil
}

func (d *FilesystemDir) GetFile(name string) (error, *FilesystemFile) {
	for _, file := range d.Files {
		if file.Name == name {
			return nil, file
		}
	}
	return errors.New("No file with name " + name), nil
}

func (d FilesystemDir) PlainName() string {
	return d.Name
}

func (d FilesystemDir) DescribeSelf() string {
	boldBlue := color.New(color.FgBlue, color.Bold)
	return boldBlue.Sprint(d.Name)
}

func (d FilesystemDir) Describe() string {
	var thingsToDescribe []Describable
	for _, file := range d.Files {
		thingsToDescribe = append(thingsToDescribe, file)
	}
	for _, dir := range d.Subdirs {
		thingsToDescribe = append(thingsToDescribe, dir)
	}
	sort.Slice(thingsToDescribe, func(i, j int) bool {
		return thingsToDescribe[i].PlainName() < thingsToDescribe[j].PlainName()
	})
	ret := ""
	for _, d := range thingsToDescribe {
		ret += d.DescribeSelf() + "  "
	}
	return ret
}

type FilesystemConfig struct {
	Root *FilesystemDir `json:"root"`
}

func fillInParents(root *FilesystemDir) {
	for _, subdir := range root.Subdirs {
		subdir.Parent = root
		fillInParents(subdir)
	}
}

func strToFilesystem(cfg []byte) FilesystemConfig {
	ret := &FilesystemConfig{}
	yaml.Unmarshal(cfg, ret)
	fillInParents(ret.Root)
	return *ret
}

var FILESYSTEM FilesystemConfig
var CURRENT_DIR = "/"

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

func getDir(cwd *FilesystemDir, path string) (error, *FilesystemDir) {
	pathParts := strings.Split(path, "/")
	var currentDir *FilesystemDir
	currentDir = cwd
	if path != "" && path[0] == '/' {
		currentDir = FILESYSTEM.Root
	}
	if path != "/" {
		for i, part := range pathParts {
			if i == 0 {
				continue
			}
			err, newDir := currentDir.GetSubdir(part)
			if err == nil {
				currentDir = newDir
			} else {
				return err, nil
			}
		}
	}
	return nil, currentDir
}

func getFileOrDir(cwd *FilesystemDir, path string) (error, Describable) {
	pathParts := strings.Split(path, "/")
	var currentDir *FilesystemDir
	currentDir = cwd
	if path != "" && path[0] == '/' {
		currentDir = FILESYSTEM.Root
	}
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

func ls(cwd *FilesystemDir, path string) (error, string) {
	err, f := getFileOrDir(cwd, path)
	if err != nil {
		return err, ""
	}
	return nil, f.Describe() + "\n"
}

func cd(cwd *FilesystemDir, path string) (error, *FilesystemDir) {
	newPath := path
	if path == "" {
		newPath = "/"
	}
	err, dir := getDir(cwd, newPath)
	if err != nil {
		return err, nil
	}
	return nil, dir
}
