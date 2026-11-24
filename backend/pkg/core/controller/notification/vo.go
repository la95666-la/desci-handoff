package notification

type ListRequest struct {
	Cursor int `json:"cursor" uri:"cursor" form:"cursor"`
}
