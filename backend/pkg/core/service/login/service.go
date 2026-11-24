package login

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/auth/sign"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/utils"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/points"
	"joincare_server/pkg/core/repository/user"
	"joincare_server/pkg/core/third/twitter"
)

func Nonce(walletAddress, address, action string) (*NonceResponse, error) {
	normalizedAddress, err := normalizeWalletAddress(firstNotEmpty(walletAddress, address))
	if err != nil {
		return nil, err
	}

	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action, err = resolveNonceAction(normalizedAddress)
		if err != nil {
			return nil, err
		}
	}
	if action != authActionLogin && action != authActionRegister {
		return nil, ret.CusError(i18n.ERR_000_00003)
	}

	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	expiredAt := time.Now().Add(authNonceTTL)
	msg := buildAuthMessage(normalizedAddress, action, nonce, expiredAt)
	record := authNonceRecord{
		WalletAddress: normalizedAddress,
		Action:        action,
		ExpiredAt:     expiredAt.Unix(),
	}

	key := fmt.Sprintf(constant.KEY_AUTH_NONCE, strings.ToLower(normalizedAddress), nonce)
	err = db.Redis().SetByJson(context.Background(), key, record, authNonceTTL)
	if err != nil {
		return nil, err
	}

	return &NonceResponse{
		WalletAddress: normalizedAddress,
		Nonce:         nonce,
		Message:       msg,
		ExpiredAt:     expiredAt.Unix(),
	}, nil
}

func WalletBindNonce(uid int, walletAddress string) (*NonceResponse, error) {
	if uid <= 0 {
		return nil, errors.New("authenticated user is required")
	}
	normalized, err := normalizeWalletAddress(walletAddress)
	if err != nil {
		return nil, err
	}
	var current entity.User
	if err := db.Mysql().Select("id,wallet_address,is_enable,session_version").Where("id = ? AND is_enable = 1", uid).Take(&current).Error; err != nil {
		return nil, errors.New("current wallet authority is unavailable")
	}
	primary, err := normalizeWalletAddress(current.WalletAddress)
	if err != nil || strings.EqualFold(primary, normalized) {
		return nil, errors.New("new wallet must differ from the current primary wallet")
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(authNonceTTL)
	message := buildWalletBindMessage(primary, normalized, uid, nonce, expires)
	record := authNonceRecord{WalletAddress: normalized, PrimaryWallet: primary, Action: authActionWalletBind, ExpiredAt: expires.Unix(), UID: uid, SessionVersion: current.SessionVersion}
	key := fmt.Sprintf(constant.KEY_AUTH_NONCE, strings.ToLower(normalized), nonce)
	if err := db.Redis().SetByJson(context.Background(), key, record, authNonceTTL); err != nil {
		return nil, err
	}
	return &NonceResponse{WalletAddress: normalized, Nonce: nonce, Message: message, ExpiredAt: expires.Unix()}, nil
}

func VerifyWalletBinding(uid int, address, message, primarySignature, newWalletSignature string) (string, error) {
	normalized, err := normalizeWalletAddress(address)
	if err != nil {
		return "", err
	}
	if uid <= 0 || strings.TrimSpace(message) == "" || strings.TrimSpace(primarySignature) == "" || strings.TrimSpace(newWalletSignature) == "" {
		return "", errors.New("wallet binding proof is incomplete")
	}
	parsed, err := parseAuthMessage(message)
	if err != nil || parsed.Action != authActionWalletBind || !strings.EqualFold(parsed.Domain, authDomain()) || time.Now().After(parsed.ExpiredAt) {
		return "", ret.CusError(i18n.ERR_001_00004)
	}
	record, raw, err := getNonceRecordRaw(normalized, parsed.Nonce)
	if err != nil || record.UID != uid || record.Action != authActionWalletBind || !strings.EqualFold(record.WalletAddress, normalized) || record.ExpiredAt != parsed.ExpiredAt.Unix() {
		return "", ret.CusError(i18n.ERR_001_00006)
	}
	var current entity.User
	if err := db.Mysql().Select("id,wallet_address,is_enable,session_version").Where("id = ? AND is_enable = 1", uid).Take(&current).Error; err != nil || current.SessionVersion != record.SessionVersion || !strings.EqualFold(current.WalletAddress, record.PrimaryWallet) {
		return "", errors.New("current wallet authority changed; request a new binding challenge")
	}
	expectedMessage := buildWalletBindMessage(record.PrimaryWallet, normalized, uid, parsed.Nonce, parsed.ExpiredAt)
	if message != expectedMessage {
		return "", errors.New("wallet binding message does not match the issued challenge")
	}
	if !verifyPersonalSign(message, primarySignature, record.PrimaryWallet) || !verifyPersonalSign(message, newWalletSignature, normalized) {
		return "", ret.CusError(i18n.ERR_001_00005)
	}
	key := fmt.Sprintf(constant.KEY_AUTH_NONCE, strings.ToLower(normalized), parsed.Nonce)
	consumed, err := db.Redis().DeleteIfValue(context.Background(), key, raw)
	if err != nil || !consumed {
		return "", ret.CusError(i18n.ERR_001_00006)
	}
	return normalized, nil
}

func buildWalletBindMessage(primary, next string, uid int, nonce string, expiredAt time.Time) string {
	return fmt.Sprintf("JoinCare wallet binding authorization\n\nCurrent Wallet: %s\nNew Wallet: %s\nUID: %d\nDomain: %s\nAction: %s\nNonce: %s\nExpiration Time: %s",
		primary, next, uid, authDomain(), authActionWalletBind, nonce, expiredAt.UTC().Format(time.RFC3339))
}

func Register(walletAddress, address, message, signature, inviteCode, clientIP string) (*LoginResponse, error) {
	normalizedAddress, err := normalizeWalletAddress(firstNotEmpty(walletAddress, address))
	if err != nil {
		return nil, err
	}
	if err = verifyAuthSignature(normalizedAddress, message, signature, authActionRegister); err != nil {
		return nil, err
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf(constant.KEY_AUTH_REGISTER_LOCK, strings.ToLower(normalizedAddress))
	if ok := db.Redis().SetNX(ctx, lockKey, "1", 15*time.Second); !ok {
		return nil, ret.CusError(i18n.ERR_001_00008)
	}
	defer db.Redis().Del(ctx, lockKey)

	userInfo := entity.User{}
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&entity.User{}).
			Select("id,wallet_address,type,is_enable,session_version").
			Where("wallet_address = ?", normalizedAddress).
			First(&userInfo).Error
		if err == nil {
			if userInfo.IsEnable != 1 {
				return errors.New("account disabled")
			}
			return completeSignedLoginTx(tx, userInfo, normalizedAddress, clientIP, now)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		userInfo = entity.User{
			WalletAddress: normalizedAddress,
			Type:          constant.UserTypeBacker,
			CreatedAt:     now,
			UpdatedAt:     now,
			LastLoginAt:   now,
			LastLoginIP:   clientIP,
		}
		// 显式 Omit telegram_id / twitter_id 让 GORM 不写这两列
		// DB schema 允许 NULL，UNIQUE 索引对 NULL 不 enforce → 多个"未绑定 tg/x"用户共存
		// 否则新用户 telegram_id='' 会跟已有空串记录冲突 uk_telegram_id
		if err = tx.Omit("telegram_id", "twitter_id").Create(&userInfo).Error; err != nil {
			return err
		}
		uid := int(userInfo.ID)
		var identity entity.UserWalletIdentity
		if err = tx.Where("wallet_address = ? AND uid = ? AND chain = 'evm'", strings.ToLower(normalizedAddress), uid).Take(&identity).Error; err != nil {
			return fmt.Errorf("verify primary wallet identity claim: %w", err)
		}
		if err = ensureSignedPrimaryWalletTx(tx, userInfo, normalizedAddress, now); err != nil {
			return err
		}
		if _, err = createInviteCode(tx, uid); err != nil {
			return err
		}
		if err = initUserPoint(tx, uid, now); err != nil {
			return err
		}
		if err = bindInviteRelation(tx, uid, inviteCode, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if userInfo.Type == constant.UserTypeBacker {
		go user.CheckUserType(int(userInfo.ID), normalizedAddress)
	}
	return buildLoginResponse(userInfo, true)
}

func Login(walletAddress, address, message, signature, clientIP string) (*LoginResponse, error) {
	normalizedAddress, err := normalizeWalletAddress(firstNotEmpty(walletAddress, address))
	if err != nil {
		return nil, err
	}
	if err = verifyAuthSignature(normalizedAddress, message, signature, authActionLogin); err != nil {
		return nil, err
	}

	userInfo, err := signedWalletLogin(db.Mysql(), normalizedAddress, clientIP)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}
	if err != nil {
		return nil, err
	}
	if userInfo.Type == constant.UserTypeBacker {
		go user.CheckUserType(int(userInfo.ID), normalizedAddress)
	}
	return buildLoginResponse(userInfo, true)
}

func RegistrationStatus(walletAddress, address string) (*RegistrationStatusResponse, error) {
	normalizedAddress, err := normalizeWalletAddress(firstNotEmpty(walletAddress, address))
	if err != nil {
		return nil, err
	}

	userInfo := entity.User{}
	err = db.Mysql().Model(&entity.User{}).
		Select("id,wallet_address,type").
		Where("wallet_address = ?", normalizedAddress).
		First(&userInfo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &RegistrationStatusResponse{
			WalletAddress: normalizedAddress,
			Registered:    false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &RegistrationStatusResponse{
		WalletAddress: normalizedAddress,
		Registered:    true,
		Uid:           int(userInfo.ID),
		InviteCode:    user.GetUserInviteCode(int(userInfo.ID)),
	}, nil
}

func AutoLogin(uid int, clientIP string) (*LoginResponse, error) {
	var userInfo entity.User
	if err := db.Mysql().Select("id,wallet_address,type,is_enable,session_version").Where("id = ?", uid).Take(&userInfo).Error; err != nil || userInfo.IsEnable != 1 {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}
	updates := map[string]interface{}{"last_login_at": time.Now()}
	if clientIP != "" {
		updates["last_login_ip"] = clientIP
	}
	_ = db.Mysql().Model(&entity.User{}).Where("id = ?", userInfo.ID).Updates(updates).Error
	return buildLoginResponse(userInfo, true)
}

func Profile(uid int) (*ProfileResponse, error) {
	userInfo := user.Info(uid)
	if userInfo.ID == 0 {
		return nil, ret.CusError(i18n.ERR_001_00007)
	}
	return &ProfileResponse{
		Uid:           int(userInfo.ID),
		WalletAddress: userInfo.WalletAddress,
		Type:          userInfo.Type,
		InviteCode:    user.GetUserInviteCode(uid),
		Point:         points.GetUserPoints(uid),
	}, nil
}

func buildLoginResponse(userInfo entity.User, registered bool) (*LoginResponse, error) {
	t, expireTime, err := sign.GenerateToken(int(userInfo.ID), userInfo.SessionVersion, viper.GetString("jwt.secret"))
	if err != nil {
		return nil, err
	}
	uid := int(userInfo.ID)
	return &LoginResponse{
		Uid:                  uid,
		Address:              userInfo.WalletAddress,
		Type:                 userInfo.Type,
		Signature:            t,
		SignatureNearExpired: int(expireTime),
		InviteCode:           user.GetUserInviteCode(uid),
		Point:                points.GetUserPoints(uid),
		Registered:           registered,
	}, nil
}

func createInviteCode(tx *gorm.DB, uid int) (string, error) {
	for i := 0; i < 3; i++ {
		code := utils.GenerateRandomWord(6, utils.Number, utils.CapitalLetter)
		insert := entity.InviteUserCode{
			UID:       uid,
			Code:      code,
			CreatedAt: time.Now(),
		}
		err := tx.Create(&insert).Error
		if err == nil {
			return code, nil
		}
	}
	return "", ret.CusError(i18n.ERR_000_00001)
}

func initUserPoint(tx *gorm.DB, uid int, now time.Time) error {
	insert := entity.UserPoint{
		UID:       uid,
		Point:     0,
		UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		DoNothing: true,
	}).Create(&insert).Error
}

func bindInviteRelation(tx *gorm.DB, uid int, inviteCode string, now time.Time) error {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return nil
	}

	var code entity.InviteUserCode
	if err := tx.Select("uid").Where("code = ?", inviteCode).Take(&code).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	inviter := code.UID
	if inviter == 0 || inviter == uid {
		return nil
	}

	// Serialize admissions for one inviter. The count-and-insert sequence below
	// must not let concurrent registrations exceed the configured daily cap.
	var inviterRow entity.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("id = ?", inviter).Take(&inviterRow).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	exists := entity.InviteRecord{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&entity.InviteRecord{}).
		Select("id").
		Where("uid = ?", uid).
		Limit(1).
		Scan(&exists).Error
	if err != nil {
		return err
	}
	if exists.ID != 0 {
		return nil
	}

	dailyCap, err := inviteDailyCap(tx)
	if err != nil {
		return err
	}
	dayStart, dayEnd := utcDayWindow(now)
	// The inviter row lock serializes all registrations for this inviter. A
	// current read is required here because Register() already read earlier in
	// the transaction and a plain COUNT could reuse an older REPEATABLE READ
	// snapshot after waiting for another registration to commit.
	var admittedToday int64
	if err := tx.Raw("SELECT COUNT(*) FROM invite_record WHERE inviter_uid = ? AND created_at >= ? AND created_at < ? FOR UPDATE", inviter, dayStart, dayEnd).
		Scan(&admittedToday).Error; err != nil {
		return err
	}
	if admittedToday >= int64(dailyCap) {
		// Registration remains successful, but this invite code is not admitted.
		// No invite_record means no later first-check-in reward can be issued.
		return nil
	}

	record := entity.InviteRecord{
		UID:        uid,
		InviterUID: inviter,
		CreatedAt:  now.UTC(),
	}
	return tx.Create(&record).Error
}

func inviteDailyCap(tx *gorm.DB) (int, error) {
	var setting entity.Settings
	err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
		Select("value").Where("`key` = ?", constant.InviteDailyCapSettingKey).Take(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return constant.InviteDailyCapDefault, nil
	}
	if err != nil {
		return 0, err
	}
	cap, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil || cap < 1 || cap > constant.InviteDailyCapMax {
		return constant.InviteDailyCapDefault, nil
	}
	return cap, nil
}

func utcDayWindow(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}

func verifyAuthSignature(address, message, signature, expectedAction string) error {
	signature = strings.TrimSpace(signature)
	if strings.TrimSpace(message) == "" || signature == "" {
		return ret.CusError(i18n.ERR_000_00003)
	}

	parsed, err := parseAuthMessage(message)
	if err != nil {
		return err
	}
	if !strings.EqualFold(parsed.Domain, authDomain()) {
		return ret.CusError(i18n.ERR_001_00004)
	}
	if parsed.Action != expectedAction {
		return ret.CusError(i18n.ERR_001_00004)
	}
	if time.Now().After(parsed.ExpiredAt) {
		return ret.CusError(i18n.ERR_001_00006)
	}
	if !strings.Contains(strings.ToLower(message), strings.ToLower(address)) {
		return ret.CusError(i18n.ERR_001_00004)
	}

	record, err := getNonceRecord(address, parsed.Nonce)
	if err != nil {
		return err
	}
	if !strings.EqualFold(record.WalletAddress, address) || record.Action != expectedAction || record.ExpiredAt != parsed.ExpiredAt.Unix() {
		return ret.CusError(i18n.ERR_001_00006)
	}

	ok := verifyPersonalSign(message, signature, address)
	if !ok {
		return ret.CusError(i18n.ERR_001_00005)
	}

	key := fmt.Sprintf(constant.KEY_AUTH_NONCE, strings.ToLower(address), parsed.Nonce)
	_ = db.Redis().Del(context.Background(), key)
	return nil
}

func getNonceRecord(address, nonce string) (authNonceRecord, error) {
	record, _, err := getNonceRecordRaw(address, nonce)
	return record, err
}

func getNonceRecordRaw(address, nonce string) (authNonceRecord, string, error) {
	key := fmt.Sprintf(constant.KEY_AUTH_NONCE, strings.ToLower(address), nonce)
	record := authNonceRecord{}
	raw, found, err := db.Redis().GetOptional(context.Background(), key)
	if err != nil || !found || json.Unmarshal([]byte(raw), &record) != nil {
		return authNonceRecord{}, "", ret.CusError(i18n.ERR_001_00006)
	}
	if record.ExpiredAt < time.Now().Unix() {
		return authNonceRecord{}, "", ret.CusError(i18n.ERR_001_00006)
	}
	return record, raw, nil
}

func parseAuthMessage(message string) (authMessage, error) {
	values := map[string]string{}
	for _, line := range strings.Split(message, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	nonce := values["nonce"]
	domain := values["domain"]
	action := strings.ToLower(values["action"])
	expiration := values["expiration time"]
	if nonce == "" || domain == "" || action == "" || expiration == "" {
		return authMessage{}, ret.CusError(i18n.ERR_001_00004)
	}

	expiredAt, err := time.Parse(time.RFC3339, expiration)
	if err != nil {
		return authMessage{}, ret.CusError(i18n.ERR_001_00004)
	}

	return authMessage{
		Domain:    domain,
		Action:    action,
		Nonce:     nonce,
		ExpiredAt: expiredAt,
	}, nil
}

func verifyPersonalSign(message, signature, expectedAddress string) bool {
	signature = strings.TrimPrefix(signature, "0x")
	signature = strings.TrimPrefix(signature, "0X")
	sigBytes, err := hex.DecodeString(signature)
	if err != nil || len(sigBytes) != crypto.SignatureLength {
		return false
	}
	if sigBytes[crypto.RecoveryIDOffset] >= 27 {
		sigBytes[crypto.RecoveryIDOffset] -= 27
	}
	if sigBytes[crypto.RecoveryIDOffset] > 1 {
		return false
	}

	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, sigBytes)
	if err != nil {
		return false
	}
	recoveredAddress := crypto.PubkeyToAddress(*pub).Hex()
	return strings.EqualFold(recoveredAddress, expectedAddress)
}

func buildAuthMessage(address, action, nonce string, expiredAt time.Time) string {
	return fmt.Sprintf("JoinCare wants you to sign in with your Ethereum account:\n%s\n\nDomain: %s\nAction: %s\nNonce: %s\nExpiration Time: %s",
		address,
		authDomain(),
		action,
		nonce,
		expiredAt.UTC().Format(time.RFC3339),
	)
}

func resolveNonceAction(address string) (string, error) {
	userInfo := entity.User{}
	err := db.Mysql().Model(&entity.User{}).
		Select("id").
		Where("wallet_address = ?", address).
		First(&userInfo).Error
	if err == nil {
		return authActionLogin, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authActionRegister, nil
	}
	return "", err
}

func newNonce() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + strconv.FormatInt(time.Now().UnixNano(), 36), nil
}

func normalizeWalletAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if !common.IsHexAddress(address) {
		return "", ret.CusError(i18n.ERR_001_00003)
	}
	return common.HexToAddress(address).Hex(), nil
}

func authDomain() string {
	domain := strings.TrimSpace(viper.GetString("auth.domain"))
	if domain == "" {
		return "joincare.network"
	}
	return domain
}

func firstNotEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func XLoginUrl() (*XLoginUrlResponse, error) {
	state, codeVerifier, authURL := twitter.GetConfig().GetAuthUrl()

	err := db.Redis().Set(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_LOGIN_VERIFIER, state), codeVerifier, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	resp := XLoginUrlResponse{
		Url: authURL,
	}
	return &resp, nil
}

func XLogin(uid int, state, code string) error {
	codeVerifier := db.Redis().Get(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_LOGIN_VERIFIER, state))
	if codeVerifier == "" {
		return ret.CusError(i18n.ERR_001_00015)
	}
	token, err := twitter.GetConfig().GetToken(code, codeVerifier)
	if err != nil {
		return ret.CusError(i18n.ERR_001_00015)
	}
	db.Redis().Del(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_LOGIN_VERIFIER, state))
	// 获取用户信息
	twInfo, err := token.Me()
	if err != nil {
		return err
	}
	twitterID := strings.TrimSpace(twInfo.Me.ID)
	if twitterID == "" {
		return ret.CusError(i18n.ERR_001_00015)
	}
	// 反作弊：同一个 X 账号在整个平台只能绑定一个 JoinCare 用户。
	// 若表上已加 uk_twitter_id 唯一索引，Updates 时 GORM 1062 也会兜住；
	// 这里前置显式判定以给出清晰业务错，且允许当前 uid 重复授权（token 续期）。
	var existing entity.User
	dupErr := db.Mysql().Model(&entity.User{}).
		Select("id").
		Where("twitter_id = ? AND id != ?", twitterID, uid).
		First(&existing).Error
	if dupErr == nil {
		return ret.CusError(i18n.ERR_001_00017)
	} else if !errors.Is(dupErr, gorm.ErrRecordNotFound) {
		return dupErr
	}
	err = db.Mysql().Model(&entity.User{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"twitter_id":           twitterID,
		"twitter_handle":       twInfo.Me.UserName,
		"twitter_avatar":       twInfo.Me.ProfileImageURL,
		"twitter_display_name": twInfo.Me.Name,
	}).Error
	if err != nil {
		return err
	}
	db.Redis().SetByJson(context.Background(), fmt.Sprintf(constant.KEY_TWITTER_INFO, uid), token, 20*86400*time.Second)
	return nil
}
