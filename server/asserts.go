package main

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed public/*
var Assets embed.FS

type Dir string

func (d Dir) Open(name string) (fs.File, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("http: invalid character in file path")
	}

	dir := string(d)
	if dir == "" {
		dir = "."
	}

	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	// If we can't find the asset, return the default index.html content
	f, err := Assets.Open(fullName)
	if os.IsNotExist(err) {
		f, err = Assets.Open(filepath.Join(dir, "index.html"))
		if err != nil {
			return nil, err
		}
	}

	// Otherwise assume this is a legitimate request routed
	// correctly
	return f, err
}
