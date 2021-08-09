package memdump

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lukjok/gipcfuzz/util"
	"github.com/pkg/errors"
)

type MemoryDumpManager interface {
	StartDump() (string, error)
}

type MemoryDump struct {
	BinaryPath    string
	DumpOutputDir string
	DumpToolPath  string
}

func NewMemoryDump(binaryPath, dumpOutputDir, dumpToolPath string) *MemoryDump {
	return &MemoryDump{
		BinaryPath:    binaryPath,
		DumpOutputDir: dumpOutputDir,
		DumpToolPath:  dumpToolPath,
	}
}

func (m *MemoryDump) StartDump() (string, error) {
	if !util.FileExists(m.DumpToolPath) {
		return "", errors.Errorf("Memory dump tool does not exist at given path: %v\n", m.DumpToolPath)
	}

	if !util.DirectoryExists(m.DumpOutputDir) {
		return "", errors.Errorf("Output directory for the memory dumps does not exist: %v\n", m.DumpOutputDir)
	}

	t := time.Now()
	execName := filepath.Base(m.BinaryPath)
	dumpFileName := fmt.Sprintf("%s_%s", execName, t.Format("20060102150405"))
	execWorkingDir := filepath.Dir(m.DumpToolPath)
	fullDumpPath := filepath.Join(m.DumpOutputDir, dumpFileName)
	toolExecParams := []string{"-accepteula", "-e", "-t", "-w", execName, fullDumpPath}

	cmd := exec.Command(m.DumpToolPath, toolExecParams...)
	cmd.Dir = execWorkingDir

	if err := cmd.Start(); err != nil {
		return "", err
	}

	return fullDumpPath, nil
}
