package watcher

import (
	"context"
	"io"

	//"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lukjok/gipcfuzz/models"
)

func IsProcessRunning(ctx context.Context) bool {
	ctxData := ctx.Value("data").(models.ContextData)
	execName := filepath.Base(ctxData.Settings.PathToExecutable)

	_, err := getProcessByName(execName)
	if err != nil {
		//log.Printf("Failed to find process with name %s", execName)
		return false
	}
	return true
}

func KillProcess(ctx context.Context) {
	ctxData := ctx.Value("data").(models.ContextData)
	execName := filepath.Base(ctxData.Settings.PathToExecutable)

	proc, err := getProcessByName(execName)
	if err != nil {
		//log.Printf("Failed to find process with name %s", execName)
	}
	proc.Kill()
}

func StartProcess(ctx context.Context, status chan *StartProcessResponse) {
	ctxData := ctx.Value("data").(models.ContextData)
	execPath := filepath.Dir(ctxData.Settings.PathToExecutable)

	//log.Printf("Starting process %s", execPath)
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

	for {
		select {
		case <-ctx.Done():
			//log.Println("Killing process...")
			cmd.Process.Kill()
			return
		default:
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				status <- NewStartProcessResponse(nil, "EXIT")
			} else {
				status <- NewStartProcessResponse(nil, buf.String())
			}
			cmd.Wait()
		}
	}
}

func getProcessByName(executableName string) (*os.Process, error) {
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

	proc, err := os.FindProcess(pid)
	return proc, err
}
