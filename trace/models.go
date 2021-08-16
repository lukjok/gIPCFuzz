package trace

type CoverageBlock struct {
	Module     string
	BlockStart uint64
	BlockEnd   uint64
}

type RPCCoverage struct {
	Module   string     `json:"module"`
	Coverage [][]string `json:"coverage"`
}
