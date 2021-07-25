package communication

type GIPCRequest struct {
	Endpoint          string
	Path              string
	Data              *string
	ProtoPath         []string
	ProtoIncludesPath []string
}
