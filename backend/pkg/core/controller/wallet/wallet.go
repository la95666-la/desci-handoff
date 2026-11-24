package wallet

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/http/tools"
	"joincare_server/pkg/core/db"
	loginService "joincare_server/pkg/core/service/login"
)

const (
	verificationPrimary = "primary_registration"
	verificationSigned  = "personal_sign"
)

type bindReq struct {
	WalletAddress      string `json:"wallet_address" binding:"required"`
	Chain              string `json:"chain"`
	Message            string `json:"message" binding:"required"`
	PrimarySignature   string `json:"primary_signature" binding:"required"`
	NewWalletSignature string `json:"new_wallet_signature" binding:"required"`
}

func normalizeEVM(address, chain string) (string, error) {
	if normalized := strings.ToLower(strings.TrimSpace(chain)); normalized != "" && normalized != "evm" {
		return "", errors.New("only EVM wallet binding is supported")
	}
	if !common.IsHexAddress(strings.TrimSpace(address)) {
		return "", errors.New("invalid EVM wallet address")
	}
	normalized := common.HexToAddress(address)
	if normalized == (common.Address{}) {
		return "", errors.New("zero wallet address is not allowed")
	}
	return normalized.Hex(), nil
}

func List(c *gin.Context) {
	uid := tools.UID(c)
	if uid == 0 {
		ret.Error(c, errors.New("not logged in"))
		return
	}
	var user entity.User
	if err := db.Mysql().Select("id,wallet_address").Where("id = ?", uid).Take(&user).Error; err != nil {
		ret.Error(c, err)
		return
	}
	if user.WalletAddress != "" {
		now := time.Now()
		row := entity.UserWallet{UID: uid, WalletAddress: common.HexToAddress(user.WalletAddress).Hex(), Chain: "evm", IsPrimary: 1, VerifiedAt: &now, VerificationMethod: verificationPrimary}
		_ = db.Mysql().Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&entity.UserWallet{}).Where("uid = ? AND LOWER(wallet_address) <> LOWER(?)", uid, row.WalletAddress).Update("is_primary", 0).Error; err != nil {
				return err
			}
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "uid"}, {Name: "wallet_address"}, {Name: "chain"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"is_primary": 1, "verified_at": gorm.Expr("COALESCE(verified_at, ?)", now), "verification_method": gorm.Expr("CASE WHEN verification_method='' THEN ? ELSE verification_method END", verificationPrimary)}),
			}).Create(&row).Error
		})
	}
	var rows []entity.UserWallet
	if err := db.Mysql().Where("uid = ?", uid).Order("is_primary DESC,id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WALLET_LIST_FAILED", "message": err.Error()})
		return
	}
	ret.Success(c, gin.H{"items": rows}, nil)
}

func BindNonce(c *gin.Context) {
	address, err := normalizeEVM(c.Query("wallet_address"), c.Query("chain"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_BAD_ADDRESS", "message": err.Error()})
		return
	}
	response, err := loginService.WalletBindNonce(tools.UID(c), address)
	if err != nil {
		ret.Error(c, err)
		return
	}
	ret.Success(c, response, nil)
}

func Bind(c *gin.Context) {
	uid := tools.UID(c)
	var req bindReq
	if uid == 0 || c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_BAD_REQUEST", "message": "authenticated signed wallet proof required"})
		return
	}
	address, err := normalizeEVM(req.WalletAddress, req.Chain)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_BAD_ADDRESS", "message": err.Error()})
		return
	}
	verified, err := loginService.VerifyWalletBinding(uid, address, req.Message, req.PrimarySignature, req.NewWalletSignature)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "WALLET_PROOF_INVALID", "message": err.Error()})
		return
	}
	now := time.Now()
	var row entity.UserWallet
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		identity := entity.UserWalletIdentity{}
		identityErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("wallet_address = ?", strings.ToLower(verified)).Take(&identity).Error
		if errors.Is(identityErr, gorm.ErrRecordNotFound) {
			identity = entity.UserWalletIdentity{WalletAddress: strings.ToLower(verified), UID: uid, Chain: "evm", ClaimMethod: verificationSigned, ClaimedAt: now}
			if err := tx.Create(&identity).Error; err != nil {
				return errors.New("wallet identity could not be claimed")
			}
		} else if identityErr != nil {
			return identityErr
		} else if identity.UID != uid || identity.Chain != "evm" {
			return errors.New("wallet already belongs to another user")
		}
		row = entity.UserWallet{UID: uid, WalletAddress: verified, Chain: "evm", VerifiedAt: &now, VerificationMethod: verificationSigned}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "uid"}, {Name: "wallet_address"}, {Name: "chain"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"verified_at": now, "verification_method": verificationSigned}),
		}).Create(&row).Error
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "code": "WALLET_OWNERSHIP_CONFLICT", "message": err.Error()})
		return
	}
	if err := db.Mysql().Where("uid = ? AND wallet_address = ? AND chain = 'evm'", uid, verified).Take(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WALLET_BIND_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": row})
}

func Unbind(c *gin.Context) {
	uid := tools.UID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_BAD_ID", "message": "invalid id"})
		return
	}
	var row entity.UserWallet
	if err := db.Mysql().Where("id = ? AND uid = ?", id, uid).Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "WALLET_NOT_FOUND", "message": "not found"})
		return
	}
	if row.IsPrimary == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_PRIMARY_LOCKED", "message": "cannot unbind primary wallet"})
		return
	}
	if err := db.Mysql().Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WALLET_UNBIND_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id}})
}

func SetPrimary(c *gin.Context) {
	uid := tools.UID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_BAD_ID", "message": "invalid id"})
		return
	}
	var selected entity.UserWallet
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND uid = ? AND chain='evm' AND verified_at IS NOT NULL", id, uid).Take(&selected).Error; err != nil {
			return errors.New("verified wallet not found")
		}
		var identity entity.UserWalletIdentity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("wallet_address = ? AND uid = ? AND chain = 'evm'", strings.ToLower(selected.WalletAddress), uid).Take(&identity).Error; err != nil {
			return errors.New("verified wallet identity ownership is unavailable")
		}
		if err := tx.Model(&entity.UserWallet{}).Where("uid = ?", uid).Update("is_primary", 0).Error; err != nil {
			return err
		}
		if result := tx.Model(&entity.UserWallet{}).Where("id = ? AND uid = ?", id, uid).Update("is_primary", 1); result.Error != nil || result.RowsAffected != 1 {
			return errors.New("wallet primary update failed")
		}
		if result := tx.Model(&entity.User{}).Where("id = ?", uid).Updates(map[string]interface{}{"wallet_address": selected.WalletAddress, "session_version": gorm.Expr("session_version + 1")}); result.Error != nil || result.RowsAffected != 1 {
			return errors.New("primary user wallet update failed")
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_SET_PRIMARY_FAILED", "message": err.Error()})
		return
	}
	_ = db.Redis().Del(context.Background(), strings.ReplaceAll(constant.KEY_USER_INFO, "%d", strconv.Itoa(uid)), strings.ReplaceAll(constant.KEY_USER_WALLET, "%d", strconv.Itoa(uid)))
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"id": id, "is_primary": 1, "reauth_required": true}})
}
