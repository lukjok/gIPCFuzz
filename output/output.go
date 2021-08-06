package output

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sync"
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
	return &Filesystem{
		OutputBaseDir: baseDir,
	}
}

func (f *Filesystem) SaveCrash(data *CrashOutput) error {
	mData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fFileName := fmt.Sprintf("%d_%s.json", data.IterationNo, data.MethodPath)
	return save(mData, path.Join(f.OutputBaseDir, CrashDirName, fFileName))
}

func (f *Filesystem) SaveProgress(data *IterationProgress) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	mData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return save(mData, path.Join(f.OutputBaseDir, ProgressFileName))
}

func save(data []byte, path string) error {
	return os.WriteFile(path, data, fs.FileMode(os.O_RDWR))
}
