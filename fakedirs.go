package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

type FileDir interface {
	Describe() string
	DescribeSelf() string
	PlainName() string
	TabcompleteName() string
	TryCD() (error, *FilesystemDir)
	TryCat() (error, string)
	Path() string
}

type FilesystemFile struct {
	Name    string         `json:"name"`
	Content string         `json:"content"`
	Parent  *FilesystemDir `json:"-"`
}

func (f FilesystemFile) PlainName() string {
	return f.Name
}

func (f FilesystemFile) TabcompleteName() string {
	return f.Name + " "
}

func (f FilesystemFile) Describe() string {
	return f.DescribeSelf()
}

func (f FilesystemFile) DescribeSelf() string {
	return f.Name
}

func (f FilesystemFile) Path() string {
	return f.Parent.Path() + "/" + f.Name
}

func (f FilesystemFile) TryCD() (error, *FilesystemDir) {
	return errors.New(fmt.Sprintf("bash: cd: %s: Not a directory", f.Name)), nil
}

func (f FilesystemFile) TryCat() (error, string) {
	return nil, f.Content
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

func (d *FilesystemDir) PathHelp(belowRoot bool) string {
	parent := d.Parent
	if parent == d {
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
	if name == "." {
		return nil, d
	}
	if name == ".." {
		return nil, d.Parent
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

func (d FilesystemDir) TabcompleteName() string {
	return d.Name + "/"
}

func (d FilesystemDir) DescribeSelf() string {
	boldBlue := color.New(color.FgBlue, color.Bold)
	return boldBlue.Sprint(d.Name)
}

func (d FilesystemDir) Describe() string {
	var thingsToDescribe []FileDir
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

func (d *FilesystemDir) TryCD() (error, *FilesystemDir) {
	return nil, d
}

func (d FilesystemDir) TryCat() (error, string) {
	return errors.New(fmt.Sprintf("cat: %s: Is a directory", d.Name)), ""
}

type FilesystemConfig struct {
	Root *FilesystemDir `json:"root"`
}

func fillInParents(root *FilesystemDir) {
	for _, subdir := range root.Subdirs {
		subdir.Parent = root
		fillInParents(subdir)
	}
	for _, file := range root.Files {
		file.Parent = root
	}
}

func strToFilesystem(cfg []byte) FilesystemConfig {
	ret := &FilesystemConfig{}
	yaml.Unmarshal(cfg, ret)
	ret.Root.Parent = ret.Root
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

func (cwd *FilesystemDir) getFileOrDir(path string) (error, FileDir) {
	if path == "" {
		return nil, cwd
	}

	pathParts := strings.Split(path, "/")
	lenParts := len(pathParts)
	leadingSlash := pathParts[0] == ""
	trailingSlash := pathParts[lenParts-1] == ""

	if lenParts == 0 {
		return nil, cwd
	}

	var currentDir *FilesystemDir = cwd

	if leadingSlash {
		currentDir = FILESYSTEM.Root
	}

	for i, part := range pathParts {
		if part == "" {
			continue
		}

		lastSegment := i == lenParts-1

		dirErr, dirRes := currentDir.GetSubdir(part)
		if dirErr == nil {
			currentDir = dirRes
			continue
		}

		if (lastSegment && trailingSlash) || !lastSegment {
			// must be a dir only
			if dirErr != nil {
				return dirErr, nil
			}
			currentDir = dirRes
			continue
		}

		// could be a file...

		fileErr, fileRes := currentDir.GetFile(part)
		if fileErr != nil {
			return errors.New(fmt.Sprintf("cannot access '%s': No such file or directory", path)), nil
		}

		return nil, fileRes
	}
	return nil, currentDir
}

func lsOne(cwd *FilesystemDir, path string) (error, string) {
	err, f := cwd.getFileOrDir(path)
	if err != nil {
		return err, ""
	}
	return nil, f.Describe() + "\n"
}

func ls(cwd *FilesystemDir, path string) (error, string) {
	trimmedPath := strings.Trim(path, " ")
	if trimmedPath == "" {
		return lsOne(cwd, "")
	}
	parts := strings.Split(trimmedPath, " ")
	errs := []string{}
	reses := []string{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		err, res := lsOne(cwd, part)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			reses = append(reses, res)
		}
	}
	resText := ""
	for _, err := range errs {
		resText += fmt.Sprintf("ls: %s\n", err)
	}
	for _, res := range reses {
		resText += res
	}
	return nil, resText
}

func cd(cwd *FilesystemDir, path string) (error, *FilesystemDir) {
	trimmedPath := strings.Trim(path, " ")
	searchPath := trimmedPath
	if trimmedPath == "" {
		searchPath = "/"
	}
	err, dir := cwd.getFileOrDir(searchPath)
	if err != nil {
		return err, nil
	}
	cdErr, cdRes := dir.TryCD()
	return cdErr, cdRes
}

func catOne(cwd *FilesystemDir, path string) (error, string) {
	err, f := cwd.getFileOrDir(path)
	if err != nil {
		return err, ""
	}
	return f.TryCat()
}

func cat(cwd *FilesystemDir, path string) (error, string) {
	trimmedPath := strings.Trim(path, " ")
	parts := strings.Split(trimmedPath, " ")
	errs := []string{}
	reses := []string{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		err, res := catOne(cwd, part)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			reses = append(reses, res)
		}
	}
	resText := ""
	for _, err := range errs {
		resText += fmt.Sprintf("cat: %s\n", err)
	}
	for _, res := range reses {
		resText += res
	}
	return nil, resText
}
