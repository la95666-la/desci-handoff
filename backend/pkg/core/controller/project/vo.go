package project

type ListRequest struct {
	Cursor int    `json:"cursor" uri:"cursor" form:"cursor"`
	Status string `json:"status" uri:"status" form:"status"`
}

type DetailRequest struct {
	ProjectID int `json:"projectID,string" uri:"projectID" form:"projectID" binding:"required"`
}

type PendingClaimListRequest struct {
	Cursor int `json:"cursor" uri:"cursor" form:"cursor"`
}

type PurchaseRecordRequest struct {
	Cursor int `json:"cursor" uri:"cursor" form:"cursor"`
}

type ClaimRecordRequest struct {
	Cursor int `json:"cursor" uri:"cursor" form:"cursor"`
}
