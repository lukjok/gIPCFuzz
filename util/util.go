package util

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/lukjok/gipcfuzz/models"
)

func GetFileNamesInDirectory(fileDir string, ignoreDirs []string) []string {
	var files []string

	err := filepath.Walk(fileDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
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

func ReadTextFile(path string) (string, error) {
	if len(path) == 0 {
		return "", errors.New("Path to the file was empty")
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Errorf("Failed to read specified file: %s", err)
	}

	return string(dat), nil
}

func DirectoryExists(path string) bool {
	if pathAbs, err := filepath.Abs(path); err != nil {
		return false
	} else if fileInfo, err := os.Stat(pathAbs); os.IsNotExist(err) || !fileInfo.IsDir() {
		return false
	}

	return true
}

func FileExists(filepath string) bool {
	fileinfo, err := os.Stat(filepath)

	if os.IsNotExist(err) {
		return false
	}
	// Return false if the fileinfo says the file path is a directory.
	return !fileinfo.IsDir()
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

func ScaleIntegers(array []int, scaleMin int, scaleMax int) {
	var nelems, i, source_min, source_max, source_scale, target_scale, zsrc, scaled int
	nelems = len(array)
	source_min = array[0]
	source_max = array[0]

	for i = 1; i < nelems; i++ {
		if array[i] < source_min {
			source_min = array[i]
		}
		if array[i] > source_max {
			source_max = array[i]
		}
	}

	if source_min == source_max {
		return
	}

	source_scale = source_max - source_min
	target_scale = scaleMax - scaleMin

	for i = 0; i < nelems; i++ {
		zsrc = array[i] - source_min
		// Round to nearest; if exactly halfway, rounds up
		scaled = (zsrc*target_scale*2 + source_scale) / source_scale / 2
		array[i] = scaled + scaleMin
	}
}

func ScaleIntegersReverse(array []int, scaleMin int, scaleMax int) {
	var nelems, i, source_min, source_max, source_scale, target_scale, zsrc, scaled int
	nelems = len(array)
	source_min = array[0]
	source_max = array[0]

	for i = 1; i < nelems; i++ {
		if array[i] < source_max {
			source_max = array[i]
		}
		if array[i] > source_min {
			source_min = array[i]
		}
	}

	if source_min == source_max {
		return
	}

	source_scale = source_max - source_min
	target_scale = scaleMax - scaleMin

	for i = 0; i < nelems; i++ {
		zsrc = array[i] - source_min
		// Round to nearest; if exactly halfway, rounds up
		scaled = (zsrc*target_scale*2 + source_scale) / source_scale / 2
		array[i] = scaled + scaleMin
	}
}
