package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/honeystats/ssh/files"
	"github.com/sirupsen/logrus"
)

var FILESYSTEM files.FilesystemConfig
var CURRENT_DIR = "/"

func init() {
	filesConfig, configSet := os.LookupEnv("FILES_CONFIG")
	if !configSet {
		panic("FILES_CONFIG is not set.")
	}

	bytes, err := ioutil.ReadFile(filesConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"filesConfig": filesConfig,
			"err":         err,
		}).Fatal("Error reading FILES_CONFIG")
	}
	FILESYSTEM = files.StrToFilesystem(bytes)
}

func lsOne(root *files.FilesystemDir, cwd *files.FilesystemDir, path string) (error, string) {
	err, f := cwd.GetFileOrDir(root, path)
	if err != nil {
		return err, ""
	}
	return nil, f.Describe() + "\n"
}

func ls(root *files.FilesystemDir, cwd *files.FilesystemDir, path string) (error, string) {
	trimmedPath := strings.Trim(path, " ")
	if trimmedPath == "" {
		return lsOne(root, cwd, "")
	}
	parts := strings.Split(trimmedPath, " ")
	errs := []string{}
	reses := []string{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		err, res := lsOne(root, cwd, part)
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

func cd(root *files.FilesystemDir, cwd *files.FilesystemDir, path string) (error, *files.FilesystemDir) {
	trimmedPath := strings.Trim(path, " ")
	searchPath := trimmedPath
	if trimmedPath == "" {
		searchPath = "/"
	}
	err, dir := cwd.GetFileOrDir(root, searchPath)
	if err != nil {
		return err, nil
	}
	cdErr, cdRes := dir.TryCD()
	return cdErr, cdRes
}

func catOne(root *files.FilesystemDir, cwd *files.FilesystemDir, path string) (error, string) {
	err, f := cwd.GetFileOrDir(root, path)
	if err != nil {
		return err, ""
	}
	return f.TryCat()
}

func cat(root *files.FilesystemDir, cwd *files.FilesystemDir, path string) (error, string) {
	trimmedPath := strings.Trim(path, " ")
	parts := strings.Split(trimmedPath, " ")
	errs := []string{}
	reses := []string{}
	for _, part := range parts {
		if part == "" {
			continue
		}
		err, res := catOne(root, cwd, part)
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
