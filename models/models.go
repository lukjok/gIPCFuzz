package models

import "github.com/lukjok/gipcfuzz/config"

type GIPCFuzzError int

const (
	Success GIPCFuzzError = iota
	NetworkError
	RequestError
	UnknownError
)

type ContextData struct {
	Settings config.Configuration
}
