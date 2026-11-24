package action

import (
	"context"
	"fmt"
	"time"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/model"
	"joincare_server/pkg/notice/db"
	"joincare_server/pkg/notice/flag"
	noticeModel "joincare_server/pkg/notice/model"
	"joincare_server/pkg/notice/repository/connection"
)

func login(addr string, uid int) {
	connection.AddUserInfo(addr, uid)
	data := model.PersonInfo{
		Uid:       uid,
		OwnerHost: fmt.Sprintf("%s:%s", flag.IP(), flag.Port()),
		Address:   addr,
	}
	db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_PERSON_INFO, uid), data, 60*time.Second)
}

func heartbeat(uid int) {
	db.Redis().Expire(context.Background(), fmt.Sprintf(constant.KEY_PERSON_INFO, uid), 60*time.Second)
	connection.Expire(uid)
	connection.SendDataByUID(uid, noticeModel.EventHeartbeat, nil)
	return
}
