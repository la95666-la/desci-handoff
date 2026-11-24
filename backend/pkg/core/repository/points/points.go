package points

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/core/db"
)

// ChangePoints keeps MySQL as the sole balance authority. The supplied
// transaction serializes one UID through a row lock and persists the resulting
// balance and flow atomically. The bool is retained for caller compatibility;
// no Redis compensation is ever required.
func ChangePoints(uid, changePoints int, changeType, desc string, tx gorm.DB) (bool, error) {
	if uid <= 0 || changePoints == 0 {
		return false, errors.New("invalid point mutation")
	}
	var current entity.UserPoint
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", uid).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		current = entity.UserPoint{UID: uid, Point: 0, UpdatedAt: time.Now()}
		if err := tx.Create(&current).Error; err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	remain := current.Point + changePoints
	if remain < 0 {
		return false, ret.CusError(i18n.ERR_001_00002)
	}
	if result := tx.Model(&entity.UserPoint{}).Where("uid = ?", uid).Updates(map[string]interface{}{"point": remain, "updated_at": time.Now()}); result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return false, result.Error
		}
		return false, errors.New("point balance row changed concurrently")
	}
	flow := entity.UserPointFlow{UID: uid, ChangePoint: changePoints, Remain: remain, Type: changeType, Desc: desc}
	if err := tx.Create(&flow).Error; err != nil {
		return false, err
	}
	return false, nil
}

func GetUserPointsFromDB(uid int) (int, bool) {
	point, err := GetUserPointsTx(db.Mysql(), uid)
	return point, err != nil
}

// GetUserPointsTx reads the balance through the caller-supplied database
// handle. Transactional callers must use this helper rather than acquiring a
// second connection from the global pool while they already hold one.
func GetUserPointsTx(tx *gorm.DB, uid int) (int, error) {
	var row entity.UserPoint
	err := tx.Select("uid,point").Where("uid = ?", uid).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Point, nil
}

func GetUserPoints(uid int) int {
	point, _ := GetUserPointsFromDB(uid)
	return point
}

// RollbackChangePoints is retained for existing call sites. Point mutation is
// now wholly inside the caller's SQL transaction, so tx.Rollback is sufficient.
func RollbackChangePoints(uid, changePoint int, changeType string, rollback, saveFlow bool) {}
