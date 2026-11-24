package notice

import (
	"encoding/json"

	"joincare_server/pkg/notice/repository/connection"
)

func Send(uid int, event string, data json.RawMessage) error {
	return connection.SendDataByUID(uid, event, data)
}

func BatchSend(uids []int, event string, data json.RawMessage) []int {
	errUIDs := make([]int, 0, len(uids))
	for _, uid := range uids {
		err := connection.SendDataByUID(uid, event, data)
		if err != nil {
			errUIDs = append(errUIDs, uid)
		}
	}
	return errUIDs
}
