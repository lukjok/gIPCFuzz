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
	Settings   config.Configuration
	UIDataChan chan *UIData
}

type UIData struct {
	StartTime           time.Time
	NewPathTime         time.Time
	LastCrashTime       time.Time
	LastHangTime        time.Time
	MessageCountInQueue int
	CyclesDone          int
	TotalPaths          int
	UniqCrash           int
	UniqHangs           int
	TotalExec           float64
	ExecSpd             float64
	CurrMsg             string
	MsgProg             float64
}
