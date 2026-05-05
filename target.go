package scanii

// Target is a Scanii regional API endpoint. Use one of the predefined
// constants or construct a custom Target with NewTarget.
//
// See https://scanii.github.io/openapi/v22/ for the full list of regional
// endpoints.
type Target struct {
	endpoint string
}

// NewTarget builds a custom Target from an explicit base URL such as
// "http://localhost:4000" (used by scanii-cli during local development).
func NewTarget(endpoint string) Target {
	return Target{endpoint: endpoint}
}

// Endpoint returns the base URL this Target points at.
func (t Target) Endpoint() string {
	return t.endpoint
}

// Predefined regional Targets.
var (
	// Deprecated: TargetAuto routes to the nearest region via latency-based
	// routing and does not guarantee which region processes your data. Use an
	// explicit regional Target (TargetUS1, TargetEU1, etc.) for data residency
	// compliance. Will be removed in a future major version.
	TargetAuto = Target{endpoint: "https://api.scanii.com"}
	TargetUS1  = Target{endpoint: "https://api-us1.scanii.com"}
	TargetEU1  = Target{endpoint: "https://api-eu1.scanii.com"}
	TargetEU2  = Target{endpoint: "https://api-eu2.scanii.com"}
	TargetAP1  = Target{endpoint: "https://api-ap1.scanii.com"}
	TargetAP2  = Target{endpoint: "https://api-ap2.scanii.com"}
	TargetCA1  = Target{endpoint: "https://api-ca1.scanii.com"}
)

const apiVersionPath = "/v2.2"

func (t Target) resolve(path string) string {
	return t.endpoint + apiVersionPath + path
}
