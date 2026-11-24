package notice

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/http/client"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/model"
	"joincare_server/pkg/core/db"
)

func Send(uid int, event string, data any) error {
	sendData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	request := model.NoticeRequest{
		Uid:   uid,
		Event: event,
		Data:  sendData,
	}
	userInfo := model.PersonInfo{}
	err = db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_PERSON_INFO, uid), &userInfo)
	if err != nil {
		return nil
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	c := client.GetHTTPClient()
	resp, err := c.Post(userInfo.OwnerHost+"/notice/v1/send", body)
	if err != nil {
		return err
	}
	r := &ret.Result{}
	err = json.Unmarshal(resp.Body(), r)
	if err != nil {
		return err
	}
	if !r.Status {
		if r.Code != "" {
			return ret.CusError(r.Code)
		} else {
			return errors.New(r.Message)
		}
	}
	return nil
}

func BatchSend(uids []int, event string, data any) error {
	sendData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(uids))
	for _, uid := range uids {
		keys = append(keys, fmt.Sprintf(constant.KEY_PERSON_INFO, uid))
	}
	sendMap := make(map[string][]int, 8)
	res := db.Redis().MGet(context.Background(), keys...)
	for _, value := range res {
		userInfo := model.PersonInfo{}
		err = json.Unmarshal([]byte(value), &userInfo)
		if err != nil {
			continue
		}
		sendMap[userInfo.OwnerHost] = append(sendMap[userInfo.OwnerHost], userInfo.Uid)
	}
	var retErr error
	for host, sendUids := range sendMap {
		request := model.BatchNoticeRequest{
			Uids:  sendUids,
			Event: event,
			Data:  sendData,
		}
		body, err := json.Marshal(request)
		if err != nil {
			retErr = err
			continue
		}
		c := client.GetHTTPClient()
		resp, err := c.Post("http://"+host+"/notice/v1/batchSend", body)
		if err != nil {
			retErr = err
			continue
		}
		r := &ret.Result{}
		err = json.Unmarshal(resp.Body(), r)
		if err != nil {
			retErr = err
			continue
		}
		if !r.Status {
			if r.Code != "" {
				retErr = ret.CusError(r.Code)
				continue
			} else {
				retErr = errors.New(r.Message)
				continue
			}
		}
	}
	return retErr
}
