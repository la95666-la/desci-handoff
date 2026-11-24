package chainevent

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
	"joincare_server/pkg/job/notice"
)

type ProjectTgeJob struct{}

func NewProjectTgeJob() model.Add {
	var job cron.Job = new(ProjectTgeJob)

	return job.(model.Add)
}

func (v *ProjectTgeJob) Add(crontab *cron.Cron) {
	crontab.AddJob("0 */10 * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
}

func (v *ProjectTgeJob) Run() {
	now := time.Now()
	tgeInfos := make([]entity.ProjectTge, 0, 8)
	err := db.Mysql().Model(&entity.ProjectTge{}).
		Select("id,project_id,`index`,vesting_percent,vesting_time,is_notification").
		Where("vesting_time <= ? and is_notification = 0", now).
		Order("vesting_time asc, id asc").
		Find(&tgeInfos).Error
	if err != nil {
		log.GetLogger().Errorf("project tge query due rows err: %s", err)
		return
	}
	for _, tgeInfo := range tgeInfos {
		if err := notifyProjectTGE(tgeInfo); err != nil {
			log.GetLogger().Errorf(
				"project tge notify err: tge_id=%d project_id=%d index=%d err=%s",
				tgeInfo.ID, tgeInfo.ProjectID, tgeInfo.Index, err,
			)
		}
	}
}

func notifyProjectTGE(tgeInfo entity.ProjectTge) error {
	projectInfos, err := GetProjectInfos([]int{tgeInfo.ProjectID})
	if err != nil {
		return err
	}
	projectInfo, ok := projectInfos[tgeInfo.ProjectID]
	if !ok {
		return fmt.Errorf("project not found")
	}
	userProjects, err := GetUserProjectInfos([]int{tgeInfo.ProjectID})
	if err != nil {
		return err
	}
	tx := db.Mysql().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	for _, info := range userProjects {
		if projectInfo.TokenPrice.IsZero() {
			tx.Rollback()
			return fmt.Errorf("project token price is zero")
		}
		released := tgeInfo.VestingPercent.Shift(-2).Mul(info.PurchaseToken.Div(projectInfo.TokenPrice)).String()
		if err := notice.CreateWithDB(tx, info.UID, int(info.ID), constant.TokensReleased, map[string][]string{
			constant.NoticeLangCn: {projectInfo.NameCn, tgeInfo.VestingPercent.String(), released, projectInfo.TokenSymbol},
			constant.NoticeLangTw: {projectInfo.NameCn, tgeInfo.VestingPercent.String(), released, projectInfo.TokenSymbol},
			constant.NoticeLangEn: {projectInfo.NameEn, tgeInfo.VestingPercent.String(), released, projectInfo.TokenSymbol},
		}); err != nil {
			tx.Rollback()
			return fmt.Errorf("persist notice for uid=%d: %w", info.UID, err)
		}
	}
	result := tx.Model(&entity.ProjectTge{}).
		Where("id = ? AND is_notification = 0", tgeInfo.ID).
		Update("is_notification", 1)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return fmt.Errorf("tge row was already handled or changed")
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	for _, info := range userProjects {
		if err := notice.Push(info.UID, int(info.ID), constant.TokensReleased); err != nil {
			log.GetLogger().Warnf("project tge realtime push failed: tge_id=%d uid=%d err=%s", tgeInfo.ID, info.UID, err)
		}
	}
	return nil
}

func GetProjectInfos(projectIDs []int) (map[int]entity.Project, error) {
	infos := make([]entity.Project, 0, len(projectIDs))
	err := db.Mysql().Model(&entity.Project{}).
		Select("project_id,owner_address,name_en,name_cn,token_address,token_symbol,token_decimals,payment_token,token_price,hard_cap,soft_cap,max_per_user,start_time,end_time,tge_time,total_raised,total_deposited,participant_count,status").
		Where("project_id in ?", projectIDs).
		Find(&infos).Error
	if err != nil {
		return nil, err
	}
	res := make(map[int]entity.Project, len(projectIDs))
	for _, info := range infos {
		res[info.ProjectID] = info
	}
	return res, nil
}

func GetUserProjectInfos(projectIDs []int) ([]entity.UserProject, error) {
	infos := make([]entity.UserProject, 0, 20)
	err := db.Mysql().
		Select("id,uid,project_id,purchase_token,next_claim_time,claim_time,claim_token,created_at").
		Model(&entity.UserProject{}).Where("project_id in ?", projectIDs).
		Find(&infos).Error
	if err != nil {
		return nil, err
	}
	return infos, nil
}
