package notification

import "joincare_server/pkg/core/repository/notification"

type ListResponse struct {
	List   []notification.Data `json:"list"`
	Cursor int                 `json:"cursor"`
}

type RedPointResponse struct {
	Light bool `json:"light"`
}
