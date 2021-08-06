package util

import (
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/lukjok/gipcfuzz/models"
)

func GetFileNamesInDirectory(fileDir string, ignoreDirs []string) []string {
	var files []string

	err := filepath.Walk(fileDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			dir := filepath.Base(path)
			for _, d := range ignoreDirs {
				if d == dir {
					return filepath.SkipDir
				}
			}
			baseDir := filepath.Base(fileDir)
			if baseDir == dir {
				return nil
			}
		}
		files = append(files, info.Name())
		return nil
	})
	if err != nil {
		log.Fatalln("Failed to enumerate files in the specified directory!")
	}

	return files
}

func GetFileFullPathInDirectory(fileDir string, ignoreDirs []string) []string {
	var files []string

	err := filepath.Walk(fileDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			dir := filepath.Base(path)
			for _, d := range ignoreDirs {
				if d == dir {
					return filepath.SkipDir
				}
			}
			baseDir := filepath.Base(fileDir)
			if baseDir == dir {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		log.Fatalln("Failed to enumerate files in the specified directory!")
	}

	return files
}

func ConvertError(err error) models.GIPCFuzzError {
	switch err.(type) {
	case nil:
		return models.Success
	case *net.OpError:
		return models.NetworkError
	default:
		return models.UnknownError
	}
}
