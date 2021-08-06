package communication

type GIPCRequest struct {
	Endpoint          string
	Path              string
	Data              *string
	ProtoFiles        []string
	ProtoIncludesPath []string
}
