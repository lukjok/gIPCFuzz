package watcher

const bufferOverflowError string = "Buffer overflow"
const memoryCorruptionError string = "Null-pointer dereference"
const unknownError string = "Unknown error"

var bufferOverflowCodes = [...]string{"0xc00000fd", "0xc0000409", "0xc0000374"}
var memoryCorruptionCodes = [...]string{"0xc0000005"}

// Process is the generic interface that is implemented on every platform
// and provides common operations for processes.
type Process interface {
	// Pid is the process ID for this process.
	Pid() int

	// PPid is the parent process ID for this process.
	PPid() int

	// Executable name running this process. This is not a path to the
	// executable.
	Executable() string
}

// Processes returns all processes.
//
// This of course will be a point-in-time snapshot of when this method was
// called. Some operating systems don't provide snapshot capability of the
// process table, in which case the process table returned might contain
// ephemeral entities that happened to be running when this was called.
func Processes() ([]Process, error) {
	return processes()
}

// FindProcess looks up a single process by pid.
//
// Process will be nil and error will be nil if a matching process is
// not found.
func FindProcess(pid int) (Process, error) {
	return findProcess(pid)
}

type StartProcessResponse struct {
	Error  error
	Output string
}

func NewStartProcessResponse(e error, output string) *StartProcessResponse {
	return &StartProcessResponse{
		Error:  e,
		Output: output,
	}
}
