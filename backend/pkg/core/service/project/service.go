package project

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"html"
	"math"
	"math/big"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/utils"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/project"
	"joincare_server/pkg/core/repository/user"
	"joincare_server/pkg/core/third/chain"
	"joincare_server/pkg/core/third/cos"
	"joincare_server/pkg/core/third/lark"
)

func List(cursor int, status, lang string) (*ListResponse, error) {
	infos := make([]entity.Project, 0, 20)
	sql := db.Mysql().
		// 加 category, cover_url, tagline_zh 供 C 端列表卡片展示（Sprint D bilingual）
		Select("id,project_id,name_en,name_cn,tagline,tagline_zh,tags,token_symbol,token_decimals,payment_token,payment_token_address,payment_token_decimals,chain_id,contract_address,contract_version,funding_mode,onchain_locked,fee_receiver,success_fee_rate,platform_fee_withdrawn,token_price,token_price_raw,token_address,hard_cap,hard_cap_raw,soft_cap,soft_cap_raw,max_per_user,max_per_user_raw,start_time,end_time,tge_time,total_raised,total_raised_raw,participant_count,status,category,cover_url,launch_at,onchain_json").
		Model(&entity.Project{}).
		Where("is_listed = ?", 1)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	if status != "" {
		if statuses, ok := publicRequestedProjectStatuses(status); ok {
			sql = sql.Where("status in ?", statuses)
		} else {
			// Explicit archived or unknown status filters must not bypass the
			// public lifecycle allowlist. Keep the response shape stable while
			// failing closed with an empty result set.
			sql = sql.Where("1 = 0")
		}
	} else {
		// Setup 對外按 PreLaunch 展示；Set 表示公開參數已完成、等待建立鏈上項目。
		// candidate 只經顯式候選區入口展示；archived 永不對外展示。
		sql = sql.Where("status in ?", publicProjectStatuses())
	}
	sql = publicV3MaintenanceScope(sql)
	err := sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	resp := &ListResponse{
		List:   make([]BriefInfo, 0, len(infos)),
		Cursor: -1,
	}
	if len(infos) <= 0 {
		return resp, nil
	}
	resp.Cursor = int(infos[len(infos)-1].ID)
	resp.List = FormatListData(lang, infos)
	return resp, nil
}

func publicProjectStatuses() []string {
	return []string{
		constant.ProjectStatusPreLaunch,
		constant.ProjectStatusSetup,
		"warmup",
		constant.ProjectStatusSet,
		constant.ProjectStatusPending,
		constant.ProjectStatusActive,
		constant.ProjectStatusSuccessfulPendingRelease,
		constant.ProjectStatusSuccessful,
		constant.ProjectStatusFailed,
		constant.ProjectStatusCanceled,
		constant.ProjectStatusEnded,
		constant.ProjectStatusRefund,
		"V3ReorgHalted",
	}
}

// A previously published V3 project remains readable during synchronization
// incidents. This status never implies transaction readiness or financial truth.
// Do not expose malformed/V2/internal projects via the maintenance exception.
func publicV3MaintenanceScope(query *gorm.DB) *gorm.DB {
	return query.Where("(status <> 'V3ReorgHalted' OR (funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND onchain_locked=1))")
}

func FormatListData(lang string, infos []entity.Project) []BriefInfo {
	funding := readPublicV3Funding(db.Mysql(), infos)
	ids := make([]int, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ProjectID)
	}
	files := project.Files(ids)
	tgeInfos := project.TGEs(ids)
	resp := make([]BriefInfo, 0, len(infos))
	cosHost, cdnKey := viper.GetString("cos.host"), viper.GetString("cdn.privateKey")
	for _, info := range infos {
		name := info.NameEn
		if useChineseProjectName(lang) {
			name = info.NameCn
		}
		file, ok := files[info.ProjectID]
		logo := entity.ProjectFile{}
		if ok {
			logo = file[constant.FileTypeLogo]
		}
		tags := make([]string, 0, 2)
		json.Unmarshal([]byte(info.Tags), &tags)

		tgeList := make([]TgeInfo, 0, len(tgeInfos[info.ProjectID]))

		for _, tge := range tgeInfos[info.ProjectID] {
			tgeList = append(tgeList, TgeInfo{
				TgeTime:         tge.VestingTime.Unix(),
				TgePercent:      tge.TgePercent,
				VestingInterval: int(tge.VestingInterval),
				VestingPercent:  tge.VestingPercent,
				VestingDuration: tge.VestingDuration,
			})
		}
		soldToken := "0"
		if !info.TokenPrice.IsZero() {
			soldToken = info.TotalRaised.Div(info.TokenPrice).String()
		}
		bi := BriefInfo{
			ProjectID: info.ProjectID,
			Name:      name,
			Logo: File{
				Url:      cos.GenerateSignedURL(cosHost, logo.Path, cdnKey),
				Size:     logo.Size,
				MimeType: logo.MimeType,
				Width:    logo.Width,
				Height:   logo.Height,
			},
			Tagline:        info.Tagline,
			Tags:           tags,
			Status:         info.Status,
			PaymentToken:   info.PaymentToken,
			TokenSymbol:    info.TokenSymbol,
			TokenAddress:   info.TokenAddress,
			SoldToken:      soldToken,
			TokenPrice:     info.TokenPrice,
			HardCap:        info.HardCap,
			SoftCap:        info.SoftCap,
			MaxPerUser:     info.MaxPerUser,
			TotalRaised:    &info.TotalRaised,
			TokenWithdrawn: utils.IntToBool(info.TokenWithdrawn),
			FundWithdrawn:  utils.IntToBool(info.FundWithdrawn),
			StartTime:      info.StartTime.Unix(),
			EndTime:        info.EndTime.Unix(),
			TgeList:        tgeList,
			Category:       info.Category,
			CoverURL:       info.CoverURL,
			TaglineZh:      info.TaglineZh,
		}
		if err := applyBriefFundingProjection(&bi, info); err != nil {
			if logger := log.GetLogger(); logger != nil {
				logger.WithError(err).WithFields(map[string]interface{}{"project_id": info.ProjectID, "contract_version": info.ContractVersion}).Error("omit project with invalid funding read projection")
			}
			continue
		}
		if needsPublicV3Funding(info) {
			bi.TotalRaised, bi.TotalRaisedRaw, bi.FundingDataStatus, bi.SoldToken = publicV3FundingValues(funding, info.ProjectID)
		}
		if info.LaunchAt != nil {
			bi.LaunchAt = info.LaunchAt.Unix()
		}
		resp = append(resp, bi)
	}
	return resp
}

func useChineseProjectName(lang string) bool {
	switch i18n.FindLang(lang) {
	case i18n.ZH_CN, i18n.ZH_TW:
		return true
	default:
		return false
	}
}

func publicRequestedProjectStatuses(status string) ([]string, bool) {
	if status == constant.ProjectStatusEnded {
		return []string{constant.ProjectStatusSuccessful, constant.ProjectStatusFailed, constant.ProjectStatusCanceled}, true
	}
	if status == "candidate" || slices.Contains(publicProjectStatuses(), status) {
		return []string{status}, true
	}
	return nil, false
}

func publicProjectDetailStatuses() []string {
	return append(publicProjectStatuses(), "candidate")
}

func publicProjectDetailStatus(status string) bool {
	return slices.Contains(publicProjectDetailStatuses(), status)
}

func exposeWarmupRegistrationCount(info entity.Project) bool {
	// Neither valid nor mismatched V3 rows may turn self-registrations into
	// a membership count. A contract-version fence is intentional here.
	return info.ContractVersion != constant.ContractVersionIDOV3Stablecoin
}

func Detail(uid, projectID int, lang string) (*DetailResponse, error) {
	info := entity.Project{}
	err := db.Mysql().
		// Sprint D 补 Phase 1 展示字段（category / cover_url / *_html / *_json / launch_at）
		Select("id,project_id,name_en,name_cn,tagline,description,tags,team_info,roadmap,partners,token_address,token_symbol,token_decimals,payment_token,payment_token_address,payment_token_decimals,chain_id,contract_address,contract_version,funding_mode,onchain_locked,fee_receiver,success_fee_rate,platform_fee_withdrawn,token_price,token_price_raw,hard_cap,hard_cap_raw,soft_cap,soft_cap_raw,max_per_user,max_per_user_raw,min_purchase_amount_raw,start_time,end_time,tge_time,total_raised,total_raised_raw,participant_count,status,created_at,category,cover_url,cover_video_url,launch_at,story_html,research_html,risks_html,tagline_zh,story_html_zh,research_html_zh,risks_html_zh,team_json,tokenomics_json,roadmap_json,documents_json,faq_json,onchain_json").
		Model(&entity.Project{}).
		Where("project_id = ? AND status IN ?", projectID, publicProjectDetailStatuses()).
		Scopes(publicV3MaintenanceScope).
		Take(&info).Error
	if err != nil {
		return nil, err
	}

	// 展示 stats：C 端不需要精确到实时，简单 COUNT 即可
	var subCnt, watchCnt, wlCnt int64
	db.Mysql().Model(&entity.ProjectSubscriber{}).Where("project_id = ?", projectID).Count(&subCnt)
	db.Mysql().Model(&entity.ProjectWatch{}).Where("project_id = ?", projectID).Count(&watchCnt)
	// Warmup self-registration is not V3 policy membership. Never surface its
	// historical row count as a V3 whitelist statistic.
	if exposeWarmupRegistrationCount(info) {
		db.Mysql().Model(&entity.ProjectWhitelist{}).Where("project_id = ?", projectID).Count(&wlCnt)
	}
	file := project.File(projectID)
	tgeInfo := project.TGE(projectID)
	name := info.NameEn
	if useChineseProjectName(lang) {
		name = info.NameCn
	}
	cosHost, cdnKey := viper.GetString("cos.host"), viper.GetString("cdn.privateKey")
	tags := make([]string, 0, 2)
	json.Unmarshal([]byte(info.Tags), &tags)

	tgeList := make([]TgeInfo, 0, len(tgeInfo))
	for _, tge := range tgeInfo {
		tgeList = append(tgeList, TgeInfo{
			TgeTime:         tge.VestingTime.Unix(),
			TgePercent:      tge.TgePercent,
			VestingInterval: int(tge.VestingInterval),
			VestingPercent:  tge.VestingPercent,
			VestingDuration: tge.VestingDuration,
		})
	}

	resp := &DetailResponse{
		Name: name,
		Logo: File{
			Url:      cos.GenerateSignedURL(cosHost, file[constant.FileTypeLogo].Path, cdnKey),
			Size:     file[constant.FileTypeLogo].Size,
			MimeType: file[constant.FileTypeLogo].MimeType,
			Width:    file[constant.FileTypeLogo].Width,
			Height:   file[constant.FileTypeLogo].Height,
		},
		Banner: File{
			Url:      cos.GenerateSignedURL(cosHost, file[constant.FileTypeBanner].Path, cdnKey),
			Size:     file[constant.FileTypeBanner].Size,
			MimeType: file[constant.FileTypeBanner].MimeType,
			Width:    file[constant.FileTypeBanner].Width,
			Height:   file[constant.FileTypeBanner].Height,
		},
		Tagline:              info.Tagline,
		Tags:                 tags,
		Description:          info.Description,
		TeamInfo:             info.TeamInfo,
		Roadmap:              info.Roadmap,
		Partners:             info.Partners,
		TokenAddress:         info.TokenAddress,
		TokenSymbol:          info.TokenSymbol,
		TokenDecimals:        info.TokenDecimals,
		PaymentToken:         info.PaymentToken,
		Status:               info.Status,
		TokenPrice:           info.TokenPrice,
		TokenPriceRaw:        info.TokenPriceRaw,
		HardCap:              info.HardCap,
		HardCapRaw:           info.HardCapRaw,
		SoftCap:              info.SoftCap,
		SoftCapRaw:           info.SoftCapRaw,
		MaxPerUser:           info.MaxPerUser,
		MaxPerUserRaw:        info.MaxPerUserRaw,
		MinPurchaseAmount:    "",
		MinPurchaseAmountRaw: "",
		TotalRaised:          &info.TotalRaised,
		TotalRaisedRaw:       info.TotalRaisedRaw,
		StartTime:            info.StartTime.Unix(),
		EndTime:              info.EndTime.Unix(),
		TgeList:              tgeList,

		// Sprint D 展示字段
		Category:       info.Category,
		CoverURL:       info.CoverURL,
		CoverVideoURL:  info.CoverVideoURL,
		StoryHTML:      info.StoryHTML,
		ResearchHTML:   info.ResearchHTML,
		RisksHTML:      info.RisksHTML,
		TaglineZh:      info.TaglineZh,
		StoryHTMLZh:    info.StoryHTMLZh,
		ResearchHTMLZh: info.ResearchHTMLZh,
		RisksHTMLZh:    info.RisksHTMLZh,
		TeamJSON:       info.TeamJSON,
		TokenomicsJSON: info.TokenomicsJSON,
		RoadmapJSON:    info.RoadmapJSON,
		DocumentsJSON:  info.DocumentsJSON,
		FAQJSON:        info.FAQJSON,
		OnchainJSON:    info.OnchainJSON,

		SubscriberCount: subCnt,
		WatchCount:      watchCnt,
		WhitelistCount:  wlCnt,
	}
	if info.MinPurchaseAmountRaw != nil {
		resp.MinPurchaseAmountRaw = *info.MinPurchaseAmountRaw
		resp.MinPurchaseAmount, err = money.FormatRaw(*info.MinPurchaseAmountRaw, uint8(info.PaymentTokenDecimals))
		if err != nil {
			return nil, fmt.Errorf("invalid minimum purchase policy: %w", err)
		}
	}
	if err := applyDetailFundingProjection(resp, info); err != nil {
		return nil, err
	}
	if needsPublicV3Funding(info) {
		resp.TotalRaised, resp.TotalRaisedRaw, resp.FundingDataStatus, _ = publicV3FundingValues(readPublicV3Funding(db.Mysql(), []entity.Project{info}), info.ProjectID)
	}
	if info.LaunchAt != nil {
		resp.LaunchAt = info.LaunchAt.Unix()
	}
	return resp, nil
}

func applyBriefV2Identity(out *BriefInfo, info entity.Project) {
	out.FundingMode, _ = constant.FundingModeForContractVersion(info.ContractVersion)
	out.PaymentTokenAddress = info.PaymentTokenAddress
	out.PaymentTokenDecimals = info.PaymentTokenDecimals
	out.ChainID = info.ChainID
	out.ContractAddress = info.ContractAddress
	out.ContractVersion = info.ContractVersion
	out.OnchainLocked = info.OnchainLocked
	out.FeeReceiver = info.FeeReceiver
	out.SuccessFeeRate = info.SuccessFeeRate
	out.PlatformFeeWithdrawn = utils.IntToBool(info.PlatformFeeWithdrawn)
	out.TokenDecimals = info.TokenDecimals
	out.TokenPriceRaw = info.TokenPriceRaw
	out.HardCapRaw = info.HardCapRaw
	out.SoftCapRaw = info.SoftCapRaw
	out.MaxPerUserRaw = info.MaxPerUserRaw
	out.TotalRaisedRaw = info.TotalRaisedRaw
}

func applyDetailV2Identity(out *DetailResponse, info entity.Project) {
	out.FundingMode, _ = constant.FundingModeForContractVersion(info.ContractVersion)
	out.PaymentTokenAddress = info.PaymentTokenAddress
	out.PaymentTokenDecimals = info.PaymentTokenDecimals
	out.ChainID = info.ChainID
	out.ContractAddress = info.ContractAddress
	out.ContractVersion = info.ContractVersion
	out.OnchainLocked = info.OnchainLocked
	out.FeeReceiver = info.FeeReceiver
	out.SuccessFeeRate = info.SuccessFeeRate
	out.PlatformFeeWithdrawn = utils.IntToBool(info.PlatformFeeWithdrawn)
	out.TokenPriceRaw = info.TokenPriceRaw
	out.HardCapRaw = info.HardCapRaw
	out.SoftCapRaw = info.SoftCapRaw
	out.MaxPerUserRaw = info.MaxPerUserRaw
	out.TotalRaisedRaw = info.TotalRaisedRaw
}

func applyBriefFundingProjection(out *BriefInfo, info entity.Project) error {
	applyBriefV2Identity(out, info)
	projection, err := ProjectFundingReadProjection(info)
	if err != nil {
		return err
	}
	out.FundingMode = projection.FundingMode
	out.ChainStatus = projection.ChainStatus
	out.SaleTokenAmount = projection.SaleTokenAmount
	out.SaleTokenAmountRaw = projection.SaleTokenAmountRaw
	if projection.FundingMode == constant.FundingModeFairLaunch {
		out.SoftCap = decimal.RequireFromString(projection.SoftCap)
		out.SoftCapRaw = projection.SoftCapRaw
		out.TotalTokensDeposited = info.TotalDeposited.String()
		out.TotalTokensDepositedRaw = info.TotalDepositedRaw
	}
	return nil
}

func applyDetailFundingProjection(out *DetailResponse, info entity.Project) error {
	applyDetailV2Identity(out, info)
	projection, err := ProjectFundingReadProjection(info)
	if err != nil {
		return err
	}
	out.FundingMode = projection.FundingMode
	out.ChainStatus = projection.ChainStatus
	out.SaleTokenAmount = projection.SaleTokenAmount
	out.SaleTokenAmountRaw = projection.SaleTokenAmountRaw
	if projection.FundingMode == constant.FundingModeFairLaunch {
		out.SoftCap = decimal.RequireFromString(projection.SoftCap)
		out.SoftCapRaw = projection.SoftCapRaw
		out.TotalTokensDeposited = info.TotalDeposited.String()
		out.TotalTokensDepositedRaw = info.TotalDepositedRaw
	}
	return nil
}

func PendingClaimList(uid, cursor int, lang string) (*PendingClaimListResponse, error) {
	now := time.Now()
	infos := make([]entity.UserProject, 0, 20)
	sql := db.Mysql().
		Select("id,uid,project_id,purchase_token,next_claim_time,claim_time,claim_token,created_at").
		Model(&entity.UserProject{}).Where("uid = ?", uid)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	err := sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil {
		return nil, err
	}
	resp := &PendingClaimListResponse{
		Cursor: -1,
		List:   nil,
	}
	if len(infos) == 0 {
		return resp, nil
	}
	resp.Cursor = int(infos[len(infos)-1].ID)
	ids := make([]int, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ProjectID)
	}
	tgeInfos := project.TGEs(ids)
	projectInfos := project.Infos(ids)
	for _, info := range infos {
		if projectInfos[info.ProjectID].Status != constant.ProjectStatusSuccessful {
			continue
		}
		name := projectInfos[info.ProjectID].NameEn
		if useChineseProjectName(lang) {
			name = projectInfos[info.ProjectID].NameCn
		}
		res := &PendingClaimData{
			Name:          name,
			ProjectID:     info.ProjectID,
			TokenAddress:  projectInfos[info.ProjectID].TokenAddress,
			TokenSymbol:   projectInfos[info.ProjectID].TokenSymbol,
			TokenDecimals: projectInfos[info.ProjectID].TokenDecimals,
			ClaimNum:      decimal.Decimal{},
			NextClaimTime: 0,
			NextClaimNum:  decimal.Decimal{},
		}
		for _, tge := range tgeInfos[info.ProjectID] {
			if tge.VestingTime.Unix() > now.Unix() {
				if res.NextClaimTime == 0 {
					res.NextClaimTime = tge.VestingTime.Unix()
					res.NextClaimNum = tge.VestingPercent.Shift(-2).Mul(info.PurchaseToken.Div(projectInfos[info.ProjectID].TokenPrice))
					break
				}
				continue
			}
			res.ClaimNum = res.ClaimNum.Add(tge.VestingPercent.Shift(-2).Mul(info.PurchaseToken.Div(projectInfos[info.ProjectID].TokenPrice)))
		}
		res.ClaimNum = res.ClaimNum.Sub(info.ClaimToken)
		resp.List = append(resp.List, *res)
	}
	return resp, nil
}

func PurchaseRecord(uid, cursor int, lang string) (*PurchaseRecordListResponse, error) {
	infos := make([]entity.UserProject, 0, 20)
	sql := db.Mysql().
		Select("id,uid,project_id,purchase_token,next_claim_time,claim_time,claim_token,is_refund,created_at").
		Model(&entity.UserProject{}).Where("uid = ?", uid)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	err := sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil {
		return nil, err
	}

	resp := &PurchaseRecordListResponse{
		Cursor: -1,
		List:   make([]PurchaseRecordData, 0, len(infos)),
	}
	if len(infos) == 0 {
		return resp, nil
	}
	resp.Cursor = int(infos[len(infos)-1].ID)
	ids := make([]int, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ProjectID)
	}

	projectInfos := project.Infos(ids)
	for _, info := range infos {
		name := projectInfos[info.ProjectID].NameEn
		if useChineseProjectName(lang) {
			name = projectInfos[info.ProjectID].NameCn
		}
		status := projectInfos[info.ProjectID].Status
		if status == constant.ProjectStatusFailed && info.IsRefund == 1 {
			status = constant.ProjectStatusRefund
		}
		resp.List = append(resp.List, PurchaseRecordData{
			Name:          name,
			ProjectID:     info.ProjectID,
			TokenAddress:  projectInfos[info.ProjectID].TokenAddress,
			TokenSymbol:   projectInfos[info.ProjectID].TokenSymbol,
			TokenDecimals: projectInfos[info.ProjectID].TokenDecimals,
			PurchaseToken: info.PurchaseToken,
			ClaimToken:    info.ClaimToken,
			Status:        projectInfos[info.ProjectID].Status,
			CreatedAt:     info.CreatedAt.Unix(),
		})
	}
	return resp, nil
}

func ClaimRecord(uid, cursor int, lang string) (*ClaimRecordResponse, error) {
	infos := make([]entity.UserClaimRecord, 0, 20)
	sql := db.Mysql().
		Select("id,uid,project_id,claim_token,block_number,tx_hash,created_at").
		Model(&entity.UserClaimRecord{}).Where("uid = ?", uid)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	err := sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil {
		return nil, err
	}

	resp := &ClaimRecordResponse{
		Cursor: -1,
		List:   make([]RecordData, 0, len(infos)),
	}
	if len(infos) == 0 {
		return resp, nil
	}
	resp.Cursor = int(infos[len(infos)-1].ID)
	ids := make([]int, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ProjectID)
	}
	projectInfos := project.Infos(ids)

	for _, info := range infos {
		name := projectInfos[info.ProjectID].NameEn
		if useChineseProjectName(lang) {
			name = projectInfos[info.ProjectID].NameCn
		}
		resp.List = append(resp.List, RecordData{
			Name:          name,
			ProjectID:     info.ProjectID,
			TokenAddress:  projectInfos[info.ProjectID].TokenAddress,
			TokenSymbol:   projectInfos[info.ProjectID].TokenSymbol,
			TokenDecimals: projectInfos[info.ProjectID].TokenDecimals,
			ClaimToken:    info.ClaimToken,
			TxHash:        info.TxHash,
			CreatedAt:     info.CreatedAt.Unix(),
		})
	}
	return resp, nil
}

type OnChainSubmission struct {
	TxHash    string `json:"tx_hash"`
	Status    string `json:"status"`
	Submitted bool   `json:"submitted"`
}

type OnChainMutationGuard func(tx *gorm.DB) error

func Create(projectID int64) (*OnChainSubmission, error) {
	return CreateWithGuard(projectID, nil)
}

func CreateWithGuard(projectID int64, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	info := entity.Project{}
	if err := db.Mysql().Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if constant.IsExactV2ProjectIdentity(info.FundingMode, info.ContractVersion) {
		return createV2(projectID, guard)
	}
	if info.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if info.FundingMode != constant.FundingModeCappedPresale {
			return nil, errors.New("V3 create requires exact capped-presale identity")
		}
		if !ApprovedV3ModeReady() {
			return nil, errors.New("V3 approved create prerequisites unavailable")
		}
		return executeApprovedV3Create(projectID, guard)
	}
	if info.ContractVersion == constant.ContractVersionV2Stablecoin {
		return nil, errors.New("project claims V2 contract version without exact capped-presale identity")
	}
	if guard != nil {
		return nil, errors.New("administrator on-chain create supports only durable V2 projects")
	}
	if info.Status != constant.ProjectStatusSet {
		return nil, fmt.Errorf("project status must be %s, got %s", constant.ProjectStatusSet, info.Status)
	}
	if err := requireLegacyRelayerVersion(info.ContractVersion); err != nil {
		return nil, err
	}
	tgeInfo := project.TGE(int(projectID))
	if len(tgeInfo) == 0 {
		return nil, errors.New("project vesting schedule is empty")
	}
	times := make([]*big.Int, 0, len(tgeInfo))
	percents := make([]*big.Int, 0, len(tgeInfo))
	for _, tge := range tgeInfo {
		times = append(times, big.NewInt(tge.VestingTime.Unix()))
		percents = append(percents, tge.VestingPercent.Shift(2).BigInt())
	}
	duration := new(big.Int)
	if len(times) > 0 {
		duration = duration.Sub(times[len(times)-1], times[0])
	}
	param := baseChain.CreateProjectParams{
		ProjectID:       projectID,
		ProjectToken:    common.HexToAddress(info.TokenAddress),
		ProjectOwner:    common.HexToAddress(info.OwnerAddress),
		TokenPrice:      info.TokenPrice.Shift(constant.Precision).BigInt(),
		HardCap:         info.HardCap.Shift(constant.Precision).BigInt(),
		SoftCap:         info.SoftCap.Shift(constant.Precision).BigInt(),
		MaxPerUser:      info.MaxPerUser.Shift(constant.Precision).BigInt(),
		StartTime:       big.NewInt(info.StartTime.Unix()),
		EndTime:         big.NewInt(info.EndTime.Unix()),
		VestingTimes:    times,
		VestingPercents: percents,
		VestingDuration: duration,
	}
	chainClient := chain.GetClient(constant.ChainProject)
	if chainClient == nil {
		return nil, errors.New("project chain client is not configured")
	}
	txHash, err := chainClient.CreateProject(param)
	if err != nil {
		return nil, err
	}
	result := db.Mysql().Model(&entity.Project{}).
		Where("project_id = ? AND status = ?", projectID, constant.ProjectStatusSet).
		Updates(map[string]interface{}{
			"status":         constant.ProjectStatusPending,
			"onchain_locked": true,
		})
	if result.Error != nil {
		return nil, fmt.Errorf("chain create submitted but project status update failed: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("chain create submitted but project status changed concurrently")
	}
	go user.ChangeUserTypeToCreator(0, info.OwnerAddress)
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusPending, Submitted: true}, nil
}

func Active(projectID int) (*OnChainSubmission, error) {
	return ActiveWithGuard(projectID, nil)
}

func ActiveWithGuard(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("project_id", "status", "contract_version", "funding_mode").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if constant.IsExactV2ProjectIdentity(info.FundingMode, info.ContractVersion) {
		return activateV2(projectID, guard)
	}
	if info.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if info.FundingMode != constant.FundingModeCappedPresale {
			return nil, errors.New("V3 activation requires exact capped-presale identity")
		}
		if !ApprovedV3ModeReady() {
			return nil, errors.New("V3 approved lifecycle prerequisites unavailable")
		}
		return executeApprovedV3Lifecycle(projectID, guard, "activateProject", "ido_v3_activate")
	}
	if info.ContractVersion == constant.ContractVersionV2Stablecoin {
		return nil, errors.New("project claims V2 contract version without exact capped-presale identity")
	}
	if guard != nil {
		return nil, errors.New("administrator on-chain activation supports only durable V2 projects")
	}
	if info.Status != constant.ProjectStatusPending {
		return nil, fmt.Errorf("project status must be %s, got %s", constant.ProjectStatusPending, info.Status)
	}
	if err := requireLegacyRelayerVersion(info.ContractVersion); err != nil {
		return nil, err
	}
	chainClient := chain.GetClient(constant.ChainProject)
	if chainClient == nil {
		return nil, errors.New("project chain client is not configured")
	}
	txHash, err := chainClient.ActivateProject(big.NewInt(int64(projectID)))
	if err != nil {
		return nil, err
	}
	result := db.Mysql().Model(&entity.Project{}).
		Where("project_id = ? AND status = ?", projectID, constant.ProjectStatusPending).
		Update("status", constant.ProjectStatusActive)
	if result.Error != nil {
		return nil, fmt.Errorf("chain activation submitted but project status update failed: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("chain activation submitted but project status changed concurrently")
	}
	return &OnChainSubmission{TxHash: txHash, Status: constant.ProjectStatusActive, Submitted: true}, nil
}

func EndWithGuard(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("project_id", "status", "contract_version", "funding_mode").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if info.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if info.FundingMode != constant.FundingModeCappedPresale {
			return nil, errors.New("V3 end requires exact capped-presale identity")
		}
		if !ApprovedV3ModeReady() {
			return nil, errors.New("V3 approved lifecycle prerequisites unavailable")
		}
		return executeApprovedV3Lifecycle(projectID, guard, "endProject", "ido_v3_end")
	}
	if err := constant.RequireExactV2ProjectIdentity(info.FundingMode, info.ContractVersion); err != nil {
		return nil, fmt.Errorf("on-chain project end supports only exact V2 projects: %w", err)
	}
	return endV2(projectID, guard)
}

func CancelWithGuard(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("project_id", "status", "contract_version", "funding_mode").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if err := constant.RequireExactV2ProjectIdentity(info.FundingMode, info.ContractVersion); err != nil {
		return nil, fmt.Errorf("on-chain project cancel supports only exact V2 projects: %w", err)
	}
	return cancelV2(projectID, guard)
}

// WithdrawPlatformFeeWithGuard 平台侧提取成功项目的平台费（owner-only，入 feeReceiver）
func WithdrawPlatformFeeWithGuard(projectID int, guard OnChainMutationGuard) (*OnChainSubmission, error) {
	info := entity.Project{}
	if err := db.Mysql().Select("project_id", "status", "contract_version", "funding_mode", "platform_fee_withdrawn").Where("project_id = ?", projectID).Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if info.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if info.FundingMode != constant.FundingModeCappedPresale {
			return nil, errors.New("V3 platform fee requires exact capped-presale identity")
		}
		if !ApprovedV3ModeReady() {
			return nil, errors.New("V3 approved lifecycle prerequisites unavailable")
		}
		return executeApprovedV3Lifecycle(projectID, guard, "withdrawPlatformFee", "ido_v3_withdraw_platform_fee")
	}
	if err := constant.RequireExactV2ProjectIdentity(info.FundingMode, info.ContractVersion); err != nil {
		return nil, fmt.Errorf("platform fee withdraw supports only exact V2 projects: %w", err)
	}
	return withdrawPlatformFeeV2(projectID, guard)
}

// PauseIDOV2 / UnpauseIDOV2 已迁移至 v2_global_pause.go 的 WithGuard durable 实现
// （安全审计 HIGH-1：禁止 relayer 私钥直发绕过 outbox）。
// IDOPausedState 读取合约全局暂停状态
func IDOPausedState() (bool, error) {
	client := chainManagerGetV2()
	if client == nil {
		return false, errors.New("IDO V2 relayer client is not configured")
	}
	return client.IsPaused()
}

func chainManagerGetV2() *baseChain.Client {
	return chain.GetClient(constant.ChainProjectV2)
}

func requireLegacyRelayerVersion(version string) error {
	if version != constant.ContractVersionV1Legacy {
		return fmt.Errorf("project contract version %q is not supported by the legacy relayer; V2 routing is not enabled", version)
	}
	return nil
}

func MyProject(uid int, lang string) (*ListResponse, error) {
	address := user.WalletAddress(uid)
	if address == "" {
		return nil, ret.CusError(i18n.ERR_001_00001)
	}
	infos := make([]entity.Project, 0, 20)
	err := db.Mysql().
		Select("id,project_id,name_en,name_cn,tagline,tags,token_symbol,token_decimals,payment_token,payment_token_address,payment_token_decimals,chain_id,contract_address,contract_version,funding_mode,onchain_locked,fee_receiver,success_fee_rate,platform_fee_withdrawn,token_price,token_price_raw,token_address,hard_cap,hard_cap_raw,soft_cap,soft_cap_raw,max_per_user,max_per_user_raw,sale_token_amount_raw,total_deposited,total_deposited_raw,start_time,end_time,tge_time,total_raised,total_raised_raw,participant_count,status").
		Model(&entity.Project{}).
		Where("owner_address = ?", address).
		Order("id desc").Find(&infos).Error
	if err != nil {
		return nil, err
	}
	resp := &ListResponse{
		List:   FormatListData(lang, infos),
		Cursor: -1,
	}
	return resp, nil
}

// ProjectInvestors routes exact V2 to its ledger and exact V3 to canonical
// financial effects. Non-owners and missing projects fail closed.
func ProjectInvestors(uid int, projectID int, page, pageSize int) (*ProjectInvestorsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	address := user.WalletAddress(uid)
	if address == "" {
		return nil, ret.CusError(i18n.ERR_001_00001)
	}
	var proj entity.Project
	if err := db.Mysql().Select("id,project_id,owner_address,payment_token,payment_token_decimals,funding_mode,contract_version").
		Model(&entity.Project{}).Where("project_id = ?", projectID).Scan(&proj).Error; err != nil {
		return nil, err
	}
	if proj.ProjectID == 0 {
		return nil, ret.CusError(i18n.ERR_000_00003)
	}
	if proj.FundingMode == constant.FundingModeCappedPresale && proj.ContractVersion == constant.ContractVersionIDOV3Stablecoin {
		if !strings.EqualFold(proj.OwnerAddress, address) {
			return nil, ret.CusError(i18n.ERR_000_00005)
		}
		return IDOV3ProjectInvestors(uid, uint64(projectID), address, page, pageSize)
	}
	if err := constant.RequireExactV2ProjectIdentity(proj.FundingMode, proj.ContractVersion); err != nil {
		return nil, err
	}
	if !strings.EqualFold(proj.OwnerAddress, address) {
		return nil, ret.CusError(i18n.ERR_000_00005)
	}
	base := db.Mysql().Model(&entity.FundLedger{}).
		Where("project_id = ? AND contract_version = ? AND entry_type = ? AND status = ?", projectID, constant.ContractVersionV2Stablecoin, fundledger.EntryPurchasePayment, fundledger.StatusCanonical)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]entity.FundLedger, 0, pageSize)
	if err := base.
		Select("id,wallet_address,amount_display,token_symbol,tx_hash,block_number,status,created_at").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	// 關聯 user 表的社交身份（X / Telegram），按錢包地址映射
	addresses := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.WalletAddress != "" {
			addresses = append(addresses, strings.ToLower(r.WalletAddress))
		}
	}
	socialByAddress := map[string]entity.User{}
	if len(addresses) > 0 {
		users := make([]entity.User, 0, len(addresses))
		if err := db.Mysql().Select("wallet_address,twitter_handle,twitter_display_name,twitter_avatar,telegram_username,telegram_first_name,telegram_last_name,telegram_avatar").
			Model(&entity.User{}).
			Where("LOWER(wallet_address) IN ?", addresses).
			Find(&users).Error; err == nil {
			for _, u := range users {
				socialByAddress[strings.ToLower(u.WalletAddress)] = u
			}
		}
	}

	items := make([]ProjectInvestorItem, 0, len(rows))
	for _, r := range rows {
		u := socialByAddress[strings.ToLower(r.WalletAddress)]
		tgName := strings.TrimSpace(strings.TrimSpace(u.TelegramFirstName) + " " + strings.TrimSpace(u.TelegramLastName))
		items = append(items, ProjectInvestorItem{
			ID:               r.ID,
			WalletAddress:    r.WalletAddress,
			AmountDisplay:    r.AmountDisplay,
			TokenSymbol:      r.TokenSymbol,
			TxHash:           r.TxHash,
			BlockNumber:      r.BlockNumber,
			Status:           r.Status,
			CreatedAt:        r.CreatedAt.Unix(),
			TwitterHandle:    u.TwitterHandle,
			TwitterName:      u.TwitterDisplayName,
			TwitterAvatar:    u.TwitterAvatar,
			TelegramUsername: u.TelegramUsername,
			TelegramName:     tgName,
			TelegramAvatar:   u.TelegramAvatar,
		})
	}
	return &ProjectInvestorsResponse{ProjectID: projectID, Items: items, Total: int(total), Page: page, PageSize: pageSize}, nil
}

func RefundRecord(uid, cursor int, lang string) (*RefundRecordResponse, error) {
	infos := make([]entity.UserRefundRecord, 0, 20)
	sql := db.Mysql().
		Select("id,uid,project_id,refund_token,block_number,tx_hash,created_at").
		Model(&entity.UserRefundRecord{}).Where("uid = ?", uid)
	if cursor > 0 {
		sql = sql.Where("id < ?", cursor)
	}
	err := sql.Order("id desc").Limit(20).Find(&infos).Error
	if err != nil {
		return nil, err
	}

	resp := &RefundRecordResponse{
		Cursor: -1,
		List:   make([]RecordData, 0, len(infos)),
	}
	if len(infos) == 0 {
		return resp, nil
	}
	resp.Cursor = int(infos[len(infos)-1].ID)
	ids := make([]int, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ProjectID)
	}
	projectInfos := project.Infos(ids)

	for _, info := range infos {
		name := projectInfos[info.ProjectID].NameEn
		if useChineseProjectName(lang) {
			name = projectInfos[info.ProjectID].NameCn
		}
		resp.List = append(resp.List, RecordData{
			Name:          name,
			ProjectID:     info.ProjectID,
			TokenAddress:  projectInfos[info.ProjectID].TokenAddress,
			TokenSymbol:   projectInfos[info.ProjectID].TokenSymbol,
			TokenDecimals: projectInfos[info.ProjectID].TokenDecimals,
			ClaimToken:    info.RefundToken,
			TxHash:        info.TxHash,
			CreatedAt:     info.CreatedAt.Unix(),
		})
	}
	return resp, nil
}

func NewProject(uid int, req NewProjectRequest) error {
	walletAddress := user.WalletAddress(uid)
	if !common.IsHexAddress(walletAddress) || common.HexToAddress(walletAddress) == (common.Address{}) {
		return errors.New("authenticated user has no valid primary wallet")
	}
	if req.WalletAddress != "" && !strings.EqualFold(req.WalletAddress, walletAddress) {
		return errors.New("application wallet must match authenticated user's primary wallet")
	}
	req.WalletAddress = walletAddress
	possessions, err := json.Marshal(req.Possessions)
	if err != nil {
		return err
	}
	supportWanted, err := json.Marshal(req.SupportWanted)
	if err != nil {
		return err
	}
	// Lark 写入变可选：config 是 dev_disabled 或 lark 侧报错，不阻塞用户提交
	// 主数据落 apply_project 表；lark 只是同步给内部审核人看的镜像
	recordID := ""
	larkAppID := viper.GetString("lark.applyHall.appID")
	if larkAppID != "" && larkAppID != "dev_disabled" {
		larkClient := lark.GetManager().NewClient(
			larkAppID,
			viper.GetString("lark.applyHall.appSecret"),
			viper.GetString("lark.applyHall.appToken"),
			viper.GetString("lark.applyHall.tableID"))
		line := map[string]interface{}{
			"Name":             req.Name,
			"City":             req.City,
			"Email":            req.Email,
			"Wechat":           req.Wechat,
			"Telegram":         req.Telegram,
			"Major":            req.Major,
			"Identity":         req.Identity,
			"Organization":     req.Organization,
			"Project Name":     req.ProjectName,
			"Project Desc":     req.ProjectDesc,
			"Project Category": req.ProjectCategory,
			"Current Step":     req.CurrentStep,
			"Possessions":      req.Possessions,
			"Apply Reason":     req.ApplyReason,
			"Support Wanted":   req.SupportWanted,
			"Project Link":     req.ProjectLink,
			"Additional Info":  req.AdditionalInfo,
			"Wallet Address":   req.WalletAddress,
		}
		if rid, larkErr := larkClient.CreateLine(line); larkErr == nil {
			recordID = rid
		} else {
			log.GetLogger().Warnf("apply project lark sync failed (continuing to DB): %v", larkErr)
		}
	}
	// recordID 为空时，用 timestamp+wallet 后 8 位兜底一个唯一 key，供 admin 端展示/引用
	if recordID == "" {
		tail := req.WalletAddress
		if len(tail) > 8 {
			tail = tail[len(tail)-8:]
		}
		recordID = fmt.Sprintf("local_%d_%s", time.Now().UnixMilli(), tail)
	}
	now := time.Now()
	insert := entity.ApplyProject{
		Name:            req.Name,
		City:            req.City,
		Email:           req.Email,
		Telegram:        req.Telegram,
		Wechat:          req.Wechat,
		Major:           req.Major,
		Identity:        req.Identity,
		Organization:    req.Organization,
		ProjectName:     req.ProjectName,
		ProjectDesc:     req.ProjectDesc,
		ProjectCategory: req.ProjectCategory,
		CurrentStep:     req.CurrentStep,
		Possessions:     string(possessions),
		ApplyReason:     req.ApplyReason,
		SupportWanted:   string(supportWanted),
		ProjectLink:     req.ProjectLink,
		AdditionalInfo:  req.AdditionalInfo,
		Status:          constant.ApplyStatusPending,
		WalletAddress:   req.WalletAddress,
		RecordID:        recordID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return db.Mysql().Create(&insert).Error
}

// CloneApprovedApplication creates a fresh application from previously approved
// application facts. This is an audited Admin-only path: it does not impersonate
// the applicant, authenticate as their wallet, approve the new application, or
// copy any V2 project/chain parameters. Callers must independently authorize
// the Admin's project-write permission.
func adminSubmittedApplicationCopy(source entity.ApplyProject, adminID int64, now time.Time) entity.ApplyProject {
	copy := source
	copy.ID = 0
	copy.Status = constant.ApplyStatusPending
	copy.RecordID = "admin_clone_" + utils.NewUUID32()
	copy.AdditionalInfo = fmt.Sprintf("Admin-submitted copy of approved application #%d by admin #%d; applicant wallet has not submitted or signed this copy. Historical source note (not authority for this project's contract version): %s",
		source.ID, adminID, source.AdditionalInfo)
	copy.CreatedAt, copy.UpdatedAt = now, now
	return copy
}

func CloneApprovedApplication(sourceID uint64, adminID int64) (uint64, error) {
	if sourceID == 0 || adminID <= 0 || db.Mysql() == nil {
		return 0, errors.New("approved application source and admin are required")
	}
	tx := db.Mysql().Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer tx.Rollback()
	var source entity.ApplyProject
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", sourceID).Take(&source).Error; err != nil {
		return 0, errors.New("approved application source unavailable")
	}
	if source.Status != constant.ApplyStatusAgree || !common.IsHexAddress(source.WalletAddress) ||
		common.HexToAddress(source.WalletAddress) == (common.Address{}) {
		return 0, errors.New("approved application source unavailable")
	}
	var pending int64
	if err := tx.Model(&entity.ApplyProject{}).Where("status = ? AND wallet_address = ? AND project_name = ?",
		constant.ApplyStatusPending, source.WalletAddress, source.ProjectName).Count(&pending).Error; err != nil {
		return 0, err
	}
	if pending != 0 {
		return 0, errors.New("an application for this wallet and project is already pending")
	}
	copy := adminSubmittedApplicationCopy(source, adminID, time.Now())
	if err := tx.Create(&copy).Error; err != nil {
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return copy.ID, nil
}

func ApproveApplication(applicationID uint64) error {
	tx := db.Mysql().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	info := entity.ApplyProject{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", applicationID).Take(&info).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("application not found")
		}
		return err
	}
	if info.Status != constant.ApplyStatusPending {
		tx.Rollback()
		return errors.New("application is not pending")
	}
	if !common.IsHexAddress(info.WalletAddress) || common.HexToAddress(info.WalletAddress) == (common.Address{}) {
		tx.Rollback()
		return errors.New("application wallet address is invalid")
	}

	insert := newProjectFromApplication(info, int(utils.SonyFlakeId()), time.Now())
	if err := createApplicationProject(tx, &insert).Error; err != nil {
		tx.Rollback()
		return err
	}
	result := tx.Model(&entity.ApplyProject{}).
		Where("id = ? AND status = ?", applicationID, constant.ApplyStatusPending).
		Update("status", constant.ApplyStatusAgree)
	if result.Error != nil || result.RowsAffected != 1 {
		tx.Rollback()
		if result.Error != nil {
			return result.Error
		}
		return errors.New("application status changed concurrently")
	}
	if err := tx.Model(&entity.User{}).
		Where("wallet_address = ?", info.WalletAddress).
		Update("type", constant.UserTypeCreator).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// applicationStoryHTML renders only the applicant's public project summary.
// Escaping matters because this field is inserted into a rich-HTML panel.
func applicationStoryHTML(description string) string {
	if strings.TrimSpace(description) == "" {
		return ""
	}
	return "<p>" + html.EscapeString(description) + "</p>"
}

func newProjectFromApplication(info entity.ApplyProject, projectID int, now time.Time) entity.Project {
	zeroTime := time.Unix(0, 0)
	return entity.Project{
		ProjectID:    projectID,
		OwnerAddress: info.WalletAddress,
		NameEn:       info.ProjectName,
		NameCn:       info.ProjectName,
		Tagline:      info.ProjectDesc,
		// The application description is applicant-supplied, not vetted research.
		// Keep the original text as description and show it in the public Story
		// panel without treating internal AdditionalInfo as publishable content.
		Category:    info.ProjectCategory,
		Description: info.ProjectDesc,
		StoryHTML:   applicationStoryHTML(info.ProjectDesc),
		// GORM includes string zero values on Create. Empty strings are not valid
		// MySQL JSON documents, even when the destination columns are nullable.
		TeamJSON:       "[]",
		TokenomicsJSON: "{}",
		RoadmapJSON:    "[]",
		DocumentsJSON:  "[]",
		FAQJSON:        "[]",
		OnchainJSON:    "[]",
		IsListed:       1,
		Status:         constant.ProjectStatusSetup,
		StartTime:      zeroTime,
		EndTime:        zeroTime,
		TgeTime:        zeroTime,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// A capped-presale draft must leave Fair Launch-only raw columns NULL. Their
// Go zero value is "", which GORM otherwise explicitly inserts and violates
// chk_projects_funding_raw_rules (the nullable DB defaults are NULL).
func createApplicationProject(tx *gorm.DB, project *entity.Project) *gorm.DB {
	return tx.Omit("SaleTokenAmountRaw", "MinContributionRaw", "MaxContributionRaw",
		"FinalTotalRaisedRaw", "FinalSaleTokenAmountRaw").Create(project)
}

func RejectApplication(applicationID uint64) error {
	result := db.Mysql().Model(&entity.ApplyProject{}).
		Where("id = ? AND status = ?", applicationID, constant.ApplyStatusPending).
		Update("status", constant.ApplyStatusReject)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("application is not pending")
	}
	return nil
}

func SubmitProjectReview(uid int, req SubmitProjectReviewRequest) error {
	walletAddress := user.WalletAddress(uid)
	if !common.IsHexAddress(walletAddress) || common.HexToAddress(walletAddress) == (common.Address{}) {
		return errors.New("authenticated user has no valid primary wallet")
	}
	if req.WalletAddress != "" && !strings.EqualFold(req.WalletAddress, walletAddress) {
		return errors.New("project wallet must match authenticated user's primary wallet")
	}
	if err := validateProjectReview(req, time.Now()); err != nil {
		return err
	}
	paymentToken, err := resolveTrustedPaymentToken(req.PaymentToken)
	if err != nil {
		return err
	}
	verifiedTokenAddress, verifiedTokenDecimals, err := verifyProjectTokensOnChain(req.TokenAddress, req.TokenDecimals, paymentToken)
	if err != nil {
		return err
	}
	req.TokenAddress = verifiedTokenAddress
	req.TokenDecimals = verifiedTokenDecimals
	rawAmounts, err := projectRawAmounts(req, uint8(paymentToken.Decimals))
	if err != nil {
		return err
	}
	req.WalletAddress = walletAddress
	c := lark.GetManager().NewClient(viper.GetString("lark.reviewHall.appID"), viper.GetString("lark.reviewHall.appSecret"), viper.GetString("lark.reviewHall.appToken"), viper.GetString("lark.reviewHall.tableID"))
	logoToken, err := uploadImage(req.Logo, c)
	if err != nil {
		return err
	}
	bannerToken := ""
	if req.Banner != "" {
		bannerToken, err = uploadImage(req.Banner, c)
		if err != nil {
			return err
		}
	}
	tags, err := json.Marshal(req.Tags)
	if err != nil {
		return err
	}
	tx := db.Mysql().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	result := tx.Model(&entity.Project{}).
		Where("project_id = ? AND owner_address = ? AND status IN ? AND onchain_locked = ?", req.ProjectID, walletAddress, []string{constant.ProjectStatusSetup, constant.ProjectStatusSet}, false).
		Updates(map[string]interface{}{
			"name_en":                req.NameEn,
			"name_cn":                req.NameCH,
			"tagline":                req.Tagline,
			"description":            req.Description,
			"tags":                   string(tags),
			"team_info":              req.TeamInfo,
			"roadmap":                req.Roadmap,
			"partners":               req.Partners,
			"token_address":          req.TokenAddress,
			"token_symbol":           req.TokenSymbol,
			"token_decimals":         req.TokenDecimals,
			"payment_token":          paymentToken.Symbol,
			"payment_token_address":  paymentToken.Address,
			"payment_token_decimals": paymentToken.Decimals,
			"chain_id":               paymentToken.ChainID,
			"contract_address":       paymentToken.Contract,
			"contract_version":       constant.ContractVersionV2Stablecoin,
			"onchain_locked":         false,
			"token_price":            req.TokenPrice,
			"token_price_raw":        rawAmounts.TokenPrice,
			"hard_cap":               req.HardCap,
			"hard_cap_raw":           rawAmounts.HardCap,
			"soft_cap":               req.SoftCap,
			"soft_cap_raw":           rawAmounts.SoftCap,
			"max_per_user":           req.MaxPerUser,
			"max_per_user_raw":       rawAmounts.MaxPerUser,
			"start_time":             time.Unix(req.StartTime, 0),
			"end_time":               time.Unix(req.EndTime, 0),
			"tge_time":               time.Unix(req.VestingTimes[0], 0),
			"total_raised":           decimal.Zero,
			"total_raised_raw":       "0",
			"total_deposited":        decimal.Zero,
			"total_deposited_raw":    "0",
			"status":                 constant.ProjectStatusSet,
		})
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return errors.New("project not found, not owned by user, already on-chain, or not in editable draft status")
	}
	if err := tx.Where("project_id = ?", req.ProjectID).Delete(&entity.ProjectTge{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("project_id = ?", req.ProjectID).Delete(&entity.ProjectFile{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	now := time.Now()
	insert := make([]entity.ProjectTge, 0, len(req.VestingTimes))
	sumPercent := decimal.Zero
	duration := big.NewInt(req.VestingTimes[len(req.VestingTimes)-1] - req.VestingTimes[0])
	lastTime := time.Time{}

	lines := make([]map[string]interface{}, 0, len(req.VestingTimes))
	for i, vestingTimeUnix := range req.VestingTimes {
		vestingPercent := decimal.NewFromFloat(req.VestingPercent[i])
		sumPercent = sumPercent.Add(vestingPercent)
		interval := 0
		vestingTime := time.Unix(vestingTimeUnix, 0)
		if !lastTime.IsZero() {
			interval = int(vestingTime.Sub(lastTime).Hours() / 24)
		}
		insert = append(insert, entity.ProjectTge{
			ProjectID:       int(req.ProjectID),
			Index:           i + 1,
			TgePercent:      sumPercent,
			VestingPercent:  vestingPercent,
			VestingInterval: uint32(interval),
			VestingTime:     vestingTime,
			VestingDuration: int(duration.Int64()),
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		lastTime = vestingTime

		lines = append(lines, map[string]interface{}{
			"Project ID":         strconv.Itoa(int(req.ProjectID)),
			"TGE Time(UTC)":      vestingTimeUnix * 1000,
			"Vesting Percent(%)": req.VestingPercent[i],
		})
	}
	err = tx.Create(&insert).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	filesInsert := []entity.ProjectFile{
		{
			ProjectID:  int(req.ProjectID),
			Type:       constant.COS_PROGECT_LOGO,
			Path:       req.Logo,
			UploadedBy: constant.UserTypeCreator,
			CreatedAt:  now,
		},
		{
			ProjectID:  int(req.ProjectID),
			Type:       constant.COS_PROGECT_BANNER,
			Path:       req.Banner,
			UploadedBy: constant.UserTypeCreator,
			CreatedAt:  now,
		},
	}
	err = tx.Create(&filesInsert).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	line := map[string]interface{}{
		"Project ID":     strconv.Itoa(int(req.ProjectID)),
		"Chinese Name":   req.NameCH,
		"English Name":   req.NameEn,
		"Logo":           []interface{}{map[string]string{"file_token": logoToken}},
		"Wallet Address": req.WalletAddress,
		"Tagline":        req.Tagline,
		"Tags":           req.Tags,
		"Description":    req.Description,
		"Team Info":      req.TeamInfo,
		"Road Map":       req.Roadmap,
		"Partners":       req.Partners,
		"Token Address":  req.TokenAddress,
		"Token Symbol":   req.TokenSymbol,
		"Token Decimals": req.TokenDecimals,
		// Lark is an operational mirror with numeric cells; DB raw/display values remain authoritative.
		"Token Price":     req.TokenPrice.InexactFloat64(),
		"Payment Token":   paymentToken.Symbol,
		"Hard Cap":        req.HardCap.InexactFloat64(),
		"Soft Cap":        req.SoftCap.InexactFloat64(),
		"Max Per User":    req.MaxPerUser.InexactFloat64(),
		"Start Time(UTC)": req.StartTime * 1000,
		"End Time(UTC)":   req.EndTime * 1000,
	}
	if bannerToken != "" {
		line["Banner"] = []interface{}{map[string]string{"file_token": bannerToken}}
	}
	_, err = c.CreateLine(line)
	if err != nil {
		tx.Rollback()
		return err
	}

	tgeClient := lark.GetManager().NewClient(viper.GetString("lark.tgeHall.appID"), viper.GetString("lark.tgeHall.appSecret"), viper.GetString("lark.tgeHall.appToken"), viper.GetString("lark.tgeHall.tableID"))
	err = tgeClient.CreateLines(lines)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func validateProjectReview(req SubmitProjectReviewRequest, now time.Time) error {
	if req.ProjectID <= 0 {
		return errors.New("project ID must be positive")
	}
	if !common.IsHexAddress(req.TokenAddress) || common.HexToAddress(req.TokenAddress) == (common.Address{}) {
		return errors.New("project token address is invalid")
	}
	if req.TokenDecimals <= 0 || req.TokenDecimals > 18 {
		return errors.New("project token decimals must be between 1 and 18")
	}
	if _, err := normalizePaymentTokenSymbol(req.PaymentToken); err != nil {
		return err
	}
	if _, err := trustedProjectImagePath(req.Logo, "logo"); err != nil {
		return fmt.Errorf("logo: %w", err)
	}
	if req.Banner != "" {
		if _, err := trustedProjectImagePath(req.Banner, "banner"); err != nil {
			return fmt.Errorf("banner: %w", err)
		}
	}
	numbers := []struct {
		name  string
		value decimal.Decimal
	}{
		{name: "token price", value: req.TokenPrice},
		{name: "hard cap", value: req.HardCap},
		{name: "soft cap", value: req.SoftCap},
		{name: "max per user", value: req.MaxPerUser},
	}
	for _, number := range numbers {
		if !number.value.IsPositive() {
			return fmt.Errorf("%s must be a positive decimal number", number.name)
		}
		if err := validateDisplayStorage(number.value); err != nil {
			return fmt.Errorf("%s: %w", number.name, err)
		}
	}
	if req.SoftCap.GreaterThan(req.HardCap) {
		return errors.New("soft cap must not exceed hard cap")
	}
	if req.MaxPerUser.GreaterThan(req.HardCap) {
		return errors.New("max per user must not exceed hard cap")
	}
	if req.StartTime <= now.Unix() {
		return errors.New("start time must be in the future")
	}
	if req.EndTime <= req.StartTime {
		return errors.New("end time must be after start time")
	}
	if len(req.VestingTimes) == 0 || len(req.VestingTimes) != len(req.VestingPercent) {
		return errors.New("vesting times and percents must be non-empty and have equal length")
	}
	if len(req.VestingTimes) > 10 {
		return errors.New("vesting schedule must not exceed 10 periods")
	}
	totalPercent := decimal.Zero
	for i, vestingTime := range req.VestingTimes {
		percent := req.VestingPercent[i]
		if math.IsNaN(percent) || math.IsInf(percent, 0) || percent <= 0 {
			return fmt.Errorf("vesting percent at index %d must be a finite positive number", i)
		}
		decimalPercent := decimal.NewFromFloat(percent)
		if !decimalPercent.Shift(2).IsInteger() {
			return fmt.Errorf("vesting percent at index %d must have at most 2 decimal places", i)
		}
		totalPercent = totalPercent.Add(decimalPercent)
		if i > 0 && vestingTime <= req.VestingTimes[i-1] {
			return errors.New("vesting times must be strictly ascending")
		}
	}
	if !totalPercent.Equal(decimal.NewFromInt(100)) {
		return errors.New("total vesting percent must equal 100")
	}
	if req.VestingTimes[0] <= req.EndTime {
		return errors.New("first vesting time must be after end time")
	}
	return nil
}

func validateDisplayStorage(value decimal.Decimal) error {
	digits := int64(len(strings.TrimPrefix(value.Coefficient().String(), "-")))
	integerDigits := digits + int64(value.Exponent())
	if integerDigits < 1 {
		integerDigits = 1
	}
	if integerDigits > 47 {
		return errors.New("display amount exceeds DECIMAL(65,18) integer range")
	}
	return nil
}

type rawProjectAmounts struct {
	TokenPrice string
	HardCap    string
	SoftCap    string
	MaxPerUser string
}

func projectRawAmounts(req SubmitProjectReviewRequest, decimals uint8) (rawProjectAmounts, error) {
	values := []struct {
		name  string
		value decimal.Decimal
		set   func(*rawProjectAmounts, string)
	}{
		{name: "token price", value: req.TokenPrice, set: func(out *rawProjectAmounts, value string) { out.TokenPrice = value }},
		{name: "hard cap", value: req.HardCap, set: func(out *rawProjectAmounts, value string) { out.HardCap = value }},
		{name: "soft cap", value: req.SoftCap, set: func(out *rawProjectAmounts, value string) { out.SoftCap = value }},
		{name: "max per user", value: req.MaxPerUser, set: func(out *rawProjectAmounts, value string) { out.MaxPerUser = value }},
	}
	out := rawProjectAmounts{}
	for _, item := range values {
		raw, err := money.DisplayToRaw(item.value, decimals)
		if err != nil {
			return rawProjectAmounts{}, fmt.Errorf("%s is invalid for %d-decimal payment token: %w", item.name, decimals, err)
		}
		item.set(&out, raw)
	}
	return out, nil
}

func uploadImage(image string, c *lark.Client) (string, error) {
	kind := "logo"
	if strings.Contains(image, "/project/banner/") {
		kind = "banner"
	}
	objectPath, err := trustedProjectImagePath(image, kind)
	if err != nil {
		return "", err
	}
	imageURL := cos.GenerateSignedURL(viper.GetString("cos.host"), objectPath, viper.GetString("cdn.privateKey"))
	if imageURL == "" {
		return "", errors.New("trusted project image URL configuration is unavailable")
	}
	token, err := c.UploadTrustedImage(imageURL, viper.GetString("cos.host"))
	if err != nil {
		log.GetLogger().Errorf("upload image err: %s", err)
		return "", err
	}
	return token, nil
}

var trustedProjectImageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}\.(?i:png|jpe?g|webp|gif)$`)

func trustedProjectImagePath(value, kind string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("image is required")
	}
	prefix := "/project/" + kind + "/"
	objectPath := value
	if parsed, err := url.Parse(value); err == nil && parsed.IsAbs() {
		trustedHost, hostErr := url.Parse(strings.TrimSpace(viper.GetString("cos.host")))
		if hostErr != nil || trustedHost.Hostname() == "" || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), trustedHost.Hostname()) || parsed.Port() != trustedHost.Port() || parsed.User != nil {
			return "", errors.New("image URL is not on the trusted COS host")
		}
		objectPath = parsed.EscapedPath()
		decoded, decodeErr := url.PathUnescape(objectPath)
		if decodeErr != nil {
			return "", errors.New("image path encoding is invalid")
		}
		objectPath = decoded
	} else if strings.Contains(value, "://") {
		return "", errors.New("image URL is invalid")
	}
	if !strings.HasPrefix(objectPath, prefix) || path.Clean(objectPath) != objectPath || strings.Contains(objectPath, "\\") {
		return "", errors.New("image object path is outside the trusted project directory")
	}
	remainder := strings.TrimPrefix(objectPath, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 || len(parts[0]) != 8 || !allDigits(parts[0]) || !trustedProjectImageName.MatchString(parts[1]) {
		return "", errors.New("image object path format is invalid")
	}
	return objectPath, nil
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
