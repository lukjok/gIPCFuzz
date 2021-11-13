package models

import (
	"time"

	"github.com/lukjok/gipcfuzz/config"
)

type GIPCFuzzError int

const (
	Success GIPCFuzzError = iota
	NetworkError
	GRPCError
	UnknownError
)

type ContextData struct {
	Settings config.Configuration
}

type UIData struct {
	StartTime     time.Time
	NewPathTime   time.Time
	LastCrashTime time.Time
	LastHangTime  time.Time
	CyclesDone    int
	TotalPaths    int
	UniqCrash     int
	UniqHangs     int
	TotalExec     int
	ExecSpd       int
	CurrMsg       string
	MsgProg       int
}
