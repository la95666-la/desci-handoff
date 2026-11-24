package model

import "encoding/json"

type NoticeRequest struct {
	Uid   int             `json:"uid"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type BatchNoticeRequest struct {
	Uids  []int           `json:"uids"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type PersonInfo struct {
	Uid       int    `json:"uid"`
	OwnerHost string `json:"ownerHost"`
	Address   string `json:"address"`
}
