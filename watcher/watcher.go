package watcher

import (
	"context"
	"io"
	"regexp"

	//"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lukjok/gipcfuzz/models"
)

func ParseErrorCode(output string) string {
	re := regexp.MustCompile(`(?m)0x[a-zA-Z0-9]{8}\b`)
	matches := re.FindAllString(output, -1)
	for i := 0; i < len(matches); i++ {
		// Windows specific: try to detect error code that is related to the memory access violation
		// TODO: This, however, may pick random memory address that starts exacly like an error code :)
		if strings.HasPrefix(strings.ToLower(matches[i]), "0xc") {
			return matches[i]
		}
	}
	// If nothing useful was found, just return first match, for now
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func ExplainErrorCode(code string) string {
	for i := 0; i < len(bufferOverflowCodes); i++ {
		if strings.ToLower(code) == bufferOverflowCodes[i] {
			return bufferOverflowError
		}
	}
	for i := 0; i < len(memoryCorruptionCodes); i++ {
		if strings.ToLower(code) == memoryCorruptionCodes[i] {
			return memoryCorruptionError
		}
	}
	return unknownError
}

func IsProcessRunning(ctx context.Context) bool {
	ctxData := ctx.Value("data").(models.ContextData)
	execName := filepath.Base(ctxData.Settings.PathToExecutable)

	_, err := getProcessByName(execName)
	return err == nil
}

func KillProcess(ctx context.Context) {
	ctxData := ctx.Value("data").(models.ContextData)
	execName := filepath.Base(ctxData.Settings.PathToExecutable)

	proc, _ := getProcessByName(execName)
	proc.Kill()
}

func StartProcess(ctx context.Context, status chan *StartProcessResponse) {
	ctxData := ctx.Value("data").(models.ContextData)
	//execPath := filepath.Dir(ctxData.Settings.PathToExecutable)
	arguments := []string{"Start-Process", ctxData.Settings.PathToExecutable}
	arguments = append(arguments, ctxData.Settings.ExecutableArguments...)

	//log.Printf("Starting process %s", execPath)
	cmd := exec.Command("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", arguments...)
	//cmd.Stderr = os.Stderr
	//cmd.Stdout = os.Stdout
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
