package login

import (
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
)

// Called ONLY after verifyAuthSignature succeeded, inside the registration or
// login transaction with the user row locked. Never called by token AutoLogin,
// proof GET, profile reads or a posted UID/address alone. Existing immutable
// ownership is mandatory; this helper cannot claim/reassign wallet identities.
func ensureSignedPrimaryWalletTx(tx *gorm.DB, user entity.User, signedAddress string, now time.Time) error {
	invalid := errors.New("verified primary wallet binding unavailable")
	if tx == nil || user.ID <= 0 || user.IsEnable != 1 || user.SessionVersion == 0 || now.IsZero() ||
		!common.IsHexAddress(signedAddress) || common.HexToAddress(signedAddress) == (common.Address{}) ||
		!strings.EqualFold(user.WalletAddress, signedAddress) {
		return invalid
	}
	wallet := common.HexToAddress(signedAddress).Hex()
	var identity []entity.UserWalletIdentity
	if err := tx.Raw(`SELECT wallet_address,uid,chain,claim_method,claimed_at FROM user_wallet_identity
 WHERE wallet_address=? FOR SHARE`, strings.ToLower(wallet)).Scan(&identity).Error; err != nil || len(identity) != 1 ||
		identity[0].UID != int(user.ID) || identity[0].WalletAddress != strings.ToLower(wallet) || identity[0].Chain != "evm" ||
		(identity[0].ClaimMethod != "primary_registration" && identity[0].ClaimMethod != "personal_sign") {
		return invalid
	}
	// Do NOT use ON DUPLICATE KEY UPDATE: the global verified-wallet unique
	// index must never turn a foreign binding conflict into a foreign update.
	var rows []entity.UserWallet
	if err := tx.Raw(`SELECT * FROM user_wallet WHERE LOWER(wallet_address)=? AND chain='evm' FOR UPDATE`, strings.ToLower(wallet)).Scan(&rows).Error; err != nil || len(rows) > 1 {
		return invalid
	}
	if len(rows) == 1 && (rows[0].UID != int(user.ID) || rows[0].ID == 0 || rows[0].Chain != "evm" || !strings.EqualFold(rows[0].WalletAddress, wallet)) {
		return invalid
	}
	if err := tx.Model(&entity.UserWallet{}).Where("uid=? AND is_primary=1 AND LOWER(wallet_address)<>?", user.ID, strings.ToLower(wallet)).Update("is_primary", 0).Error; err != nil {
		return invalid
	}
	if len(rows) == 0 {
		row := entity.UserWallet{UID: int(user.ID), WalletAddress: wallet, Chain: "evm", IsPrimary: 1, VerifiedAt: &now, VerificationMethod: "primary_registration", BoundAt: now, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return invalid
		}
	} else {
		row := rows[0]
		updates := map[string]interface{}{"is_primary": int8(1)}
		if row.VerifiedAt == nil {
			if row.VerificationMethod != "" {
				return invalid
			}
			updates["verified_at"], updates["verification_method"] = now, "primary_registration"
		} else if row.VerificationMethod != "primary_registration" && row.VerificationMethod != "personal_sign" {
			return invalid
		}
		if err := tx.Model(&entity.UserWallet{}).Where("id=? AND uid=? AND chain='evm'", row.ID, user.ID).Updates(updates).Error; err != nil {
			return invalid
		}
	}
	// Read back the exact ownership/proof predicate before login can commit.
	var count int64
	if err := tx.Raw(`SELECT COUNT(*) FROM user_wallet w JOIN user_wallet_identity i
 ON i.uid=w.uid AND BINARY i.wallet_address=BINARY LOWER(w.wallet_address) AND i.chain=w.chain
 WHERE w.uid=? AND LOWER(w.wallet_address)=? AND w.chain='evm' AND w.is_primary=1
 AND w.verified_at IS NOT NULL AND w.verification_method IN ('primary_registration','personal_sign')`, user.ID, strings.ToLower(wallet)).Scan(&count).Error; err != nil || count != 1 {
		return invalid
	}
	return nil
}

// Re-authentication and restoration of a missing primary binding commit together.
// No login token is built until this transaction has successfully committed.
func signedWalletLogin(database *gorm.DB, address, clientIP string) (entity.User, error) {
	var user entity.User
	if database == nil {
		return user, errors.New("login database unavailable")
	}
	err := database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,wallet_address,type,is_enable,session_version").Where("wallet_address = ?", address).First(&user).Error; err != nil {
			return err
		}
		return completeSignedLoginTx(tx, user, address, clientIP, time.Now())
	})
	return user, err
}

func completeSignedLoginTx(tx *gorm.DB, user entity.User, address, clientIP string, now time.Time) error {
	if err := ensureSignedPrimaryWalletTx(tx, user, address, now); err != nil {
		return err
	}
	updates := map[string]interface{}{"last_login_at": now}
	if clientIP != "" {
		updates["last_login_ip"] = clientIP
	}
	return tx.Model(&entity.User{}).Where("id = ?", user.ID).Updates(updates).Error
}
