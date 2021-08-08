package watcher

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lukjok/gipcfuzz/models"
)

var targetPid int = 0

func IsProcessRunning(ctx context.Context) bool {
	ctxData := ctx.Value("data").(models.ContextData)
	execName := filepath.Base(ctxData.Settings.PathToExecutable)

	_, err := getProcessByName(execName)
	if err != nil {
		log.Printf("Failed to find process with name %s", execName)
		targetPid = 0
		return false
	}
	return true
}

func StartProcess(ctx context.Context, status chan *StartProcessResponse) {
	ctxData := ctx.Value("data").(models.ContextData)
	execPath := filepath.Dir(ctxData.Settings.PathToExecutable)

	cmd := exec.Command(ctxData.Settings.PathToExecutable, ctxData.Settings.ExecutableArguments...)
	cmd.Dir = execPath
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		status <- NewStartProcessResponse(err, "")
		return
	}

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, stderr); err != nil {
		status <- NewStartProcessResponse(err, "")
		return
	}

	cmd.Wait()
	status <- NewStartProcessResponse(nil, buf.String())
}

func getProcessByName(executableName string) (*os.Process, error) {
	if targetPid == 0 {
		procList, err := processes()
		if err != nil {
			return nil, err
		}
		var pid int = 0
		for _, value := range procList {
			if value.Executable() == executableName {
				pid = value.Pid()
				break
			}
		}
		targetPid = pid
	}

	proc, err := os.FindProcess(targetPid)
	return proc, err
}
