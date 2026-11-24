package notice

const (
	ReadPoint = "readPoint"
)

type ReadPointSt struct {
	Type      string `json:"type"`
	RelatedID int    `json:"relatedID,omitempty"`
}
