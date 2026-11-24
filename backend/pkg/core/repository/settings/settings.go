package settings

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/core/db"
)

func ConfigFromDB(key string) string {
	res := entity.Settings{}
	err := db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_SETTINGS, key), &res)
	if err != nil {
		err = db.Mysql().Table(res.TableName()).
			Select("`key`,`value`,`type`").Where("`key` = ?", key).Scan(&res).Error
		if err != nil {
			return ""
		}
		db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_SETTINGS, key), &res, 86400*time.Second)
	}
	return res.Value
}

func ConfigFromDBToInt(key string) int {
	value := ConfigFromDB(key)
	res, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return res
}

func CopywritingFromDB(key string, lang string) string {
	lang = string(i18n.FindLang(lang))
	res := entity.CopywritingConfig{}
	err := db.Redis().GetByJson(context.Background(), fmt.Sprintf(constant.KEY_COPYWRITING_CONFIG, key, lang), &res)
	if err != nil {
		err = db.Mysql().Table(res.TableName()).Where("`key` = ? and language = ?", key, lang).Scan(&res).Error
		if err != nil {
			return ""
		}
		db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_COPYWRITING_CONFIG, key, lang), &res, 86400*time.Second)
	}
	if res.Value == "" && lang == constant.NoticeLangTw {
		return CopywritingFromDB(key, constant.NoticeLangCn)
	}
	return res.Value
}

func PlatformFee() decimal.Decimal {
	res, _ := strconv.Atoi(ConfigFromDB("PlatformFee"))
	fee := decimal.NewFromInt(int64(res))
	return fee
}
