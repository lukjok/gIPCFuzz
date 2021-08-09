package output

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lukjok/gipcfuzz/util"
)

const (
	CrashDirName     = "Crashes"
	ProgressFileName = "progress.json"
)

type OutputManager interface {
	SaveCrash(*CrashOutput) error
	SaveProgress(*IterationProgress) error
}

type Filesystem struct {
	OutputBaseDir string
	mu            sync.Mutex
}

func NewFilesystem(baseDir string) *Filesystem {
	createBaseDirectoriesIfNotExists(baseDir)
	return &Filesystem{
		OutputBaseDir: baseDir,
	}
}

func (f *Filesystem) SaveCrash(data *CrashOutput) error {
	mData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	pathNoSuffix := strings.Replace(data.MethodPath, "/", "_", 1)
	fFileName := fmt.Sprintf("%d_%s.json", data.IterationNo, pathNoSuffix)
	return save(mData, filepath.Join(f.OutputBaseDir, CrashDirName, fFileName))
}

func (f *Filesystem) SaveProgress(data *IterationProgress) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	mData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return save(mData, filepath.Join(f.OutputBaseDir, ProgressFileName))
}

func save(data []byte, path string) error {
	return os.WriteFile(path, data, 0600)
}

func createBaseDirectoriesIfNotExists(baseDir string) {
	if util.DirectoryExists(baseDir) && util.DirectoryExists(filepath.Join(baseDir, CrashDirName)) {
		return
	}

	if err := os.Mkdir(baseDir, fs.FileMode(os.O_RDWR)); err != nil {
		log.Printf("Error while creating base output directory: %s", err)
		return
	}

	if err := os.Mkdir(filepath.Join(baseDir, CrashDirName), fs.FileMode(os.O_RDWR)); err != nil {
		log.Printf("Error while creating crash output directory: %s", err)
		return
	}
}
