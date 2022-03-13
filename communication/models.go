package communication

type GIPCRequest struct {
	Endpoint          string
	Path              string
	Data              []byte
	ProtoFiles        []string
	ProtoIncludesPath []string
}
