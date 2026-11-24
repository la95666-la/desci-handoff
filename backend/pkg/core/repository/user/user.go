package user

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"time"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/utils"
	"joincare_server/pkg/core/db"
)

func Infos(uids []int) map[int]entity.User {
	if len(uids) == 0 {
		return map[int]entity.User{}
	}
	keys := make([]string, len(uids))
	for _, uid := range uids {
		keys = append(keys, fmt.Sprintf(constant.KEY_USER_INFO, uid))
	}
	values := db.Redis().MGet(context.Background(), keys...)
	result := make(map[int]entity.User, len(values))
	notInRedis := make([]int, 0, len(values))
	for _, uid := range uids {
		key := fmt.Sprintf(constant.KEY_USER_INFO, uid)
		value, ok := values[key]
		if !ok {
			notInRedis = append(notInRedis, uid)
			continue
		}
		res := entity.User{}
		err := json.Unmarshal([]byte(value), &res)
		if err != nil {
			notInRedis = append(notInRedis, uid)
			continue
		}
		result[uid] = res
	}
	if len(notInRedis) == 0 {
		return result
	}
	// 从数据库中获取相关数据
	infos := make([]entity.User, 0, len(notInRedis))
	err := db.Mysql().Model(&entity.User{}).
		Select("id,wallet_address").
		Where("id in ?", notInRedis).
		Find(&infos).Error
	if err != nil {
		return result
	}
	for _, info := range infos {
		result[int(info.ID)] = info
		db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_USER_INFO, info.ID), info, 86400*time.Second)
	}
	return result
}

func Info(uid int) entity.User {
	res := entity.User{}
	err := db.Mysql().Model(&entity.User{}).
		Select("id,wallet_address,type").
		Where("id = ?", uid).
		First(&res).Error
	if err != nil {
		return entity.User{}
	}
	return res
}

func WalletAddress(uid int) string {
	res := entity.User{}
	err := db.Mysql().Model(&entity.User{}).Select("wallet_address").Where("id = ?", uid).Scan(&res).Error
	if err != nil {
		return ""
	}
	return res.WalletAddress
}

func ChangeUserTypeToCreator(uid int, walletAddress string) {
	if uid == 0 {
		info := &entity.User{}
		db.Mysql().Select("id").Model(&entity.User{}).Where("wallet_address = ?", walletAddress).Scan(&info)
		uid = int(info.ID)
	}
	db.Mysql().Model(&entity.User{}).Where("id = ?", uid).Update("type", constant.UserTypeCreator)
}

func CheckUserType(uid int, walletAddress string) {
	if walletAddress == "" {
		walletAddress = WalletAddress(uid)
		if walletAddress == "" {
			return
		}
	}
	// 获取是否有发布过项目
	info := &entity.Project{}
	db.Mysql().Select("id").Model(&entity.Project{}).Where("owner_address = ?", walletAddress).Limit(1).Scan(&info)
	if info.ID != 0 {
		ChangeUserTypeToCreator(uid, walletAddress)
	}
}

func NewInviteCode(uid int) string {
	code := ""
	for i := 0; i < 3; i++ {
		code = utils.GenerateRandomWord(6, utils.Number, utils.CapitalLetter)
		insert := entity.InviteUserCode{
			UID:       uid,
			Code:      code,
			CreatedAt: time.Now(),
		}
		err := db.Mysql().Create(&insert).Error
		if err == nil {
			break
		}
	}
	return code
}

func GetUserInviteCode(uid int) string {
	info := entity.InviteUserCode{}
	db.Mysql().Select("uid, code").Model(&entity.InviteUserCode{}).
		Where("uid = ?", uid).Limit(1).Scan(&info)
	if info.Code != "" {
		return info.Code
	}
	return NewInviteCode(uid)
}

func GetUidByInviteCode(code string) int {
	info := entity.InviteUserCode{}
	db.Mysql().Select("uid, code").Model(&entity.InviteUserCode{}).
		Where("code = ?", code).Limit(1).Scan(&info)
	return info.UID
}
