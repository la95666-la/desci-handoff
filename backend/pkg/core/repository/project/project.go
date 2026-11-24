package project

import (
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/core/db"
)

func Files(projectIDs []int) map[int]map[string]entity.ProjectFile {
	res := make(map[int]map[string]entity.ProjectFile, len(projectIDs))
	infos := make([]entity.ProjectFile, 0, len(projectIDs)*2)
	err := db.Mysql().Model(&entity.ProjectFile{}).
		Select("project_id,type,path,size,mime_type,width,height").
		Where("project_id in ?", projectIDs).
		Find(&infos).Error
	if err != nil {
		return res
	}
	for _, info := range infos {
		if res[info.ProjectID] == nil {
			res[info.ProjectID] = make(map[string]entity.ProjectFile, 2)
		}
		res[info.ProjectID][info.Type] = info
	}
	return res
}

func File(projectID int) map[string]entity.ProjectFile {
	res := make(map[string]entity.ProjectFile, 2)
	infos := make([]entity.ProjectFile, 0, 2)
	err := db.Mysql().Model(&entity.ProjectFile{}).
		Select("project_id,type,path,size,mime_type,width,height").
		Where("project_id = ?", projectID).
		Find(&infos).Error
	if err != nil {
		return res
	}
	for _, info := range infos {
		res[info.Type] = info
	}
	return res
}

func TGEs(projectIDs []int) map[int][]entity.ProjectTge {
	res := make(map[int][]entity.ProjectTge, len(projectIDs))
	infos := make([]entity.ProjectTge, 0, len(projectIDs)*8)
	err := db.Mysql().Model(&entity.ProjectTge{}).
		Select("project_id,`index`,tge_percent,vesting_percent,vesting_interval,vesting_time,vesting_duration,created_at").
		Where("project_id in ?", projectIDs).
		Order("project_id desc, `index` asc").
		Find(&infos).Error
	if err != nil {
		return res
	}
	for _, info := range infos {
		if res[info.ProjectID] == nil {
			res[info.ProjectID] = make([]entity.ProjectTge, 0, 8)
		}
		res[info.ProjectID] = append(res[info.ProjectID], info)
	}
	return res
}

func TGE(projectID int) []entity.ProjectTge {
	res := make([]entity.ProjectTge, 0, 8)
	infos := make([]entity.ProjectTge, 0, 8)
	err := db.Mysql().Model(&entity.ProjectTge{}).
		Select("project_id,`index`,tge_percent,vesting_percent,vesting_interval,vesting_time,vesting_duration,created_at").
		Where("project_id = ?", projectID).
		Order("`index` asc").
		Find(&infos).Error
	if err != nil {
		return res
	}
	for _, info := range infos {
		res = append(res, info)
	}
	return res
}

func Infos(projectIDs []int) map[int]entity.Project {
	infos := make([]entity.Project, 0, len(projectIDs))
	err := db.Mysql().
		Select("id,project_id,name_en,name_cn,tagline,description,tags,team_info,roadmap,partners,token_address,token_symbol,token_decimals,payment_token,payment_token_address,payment_token_decimals,chain_id,contract_address,contract_version,onchain_locked,fee_receiver,success_fee_rate,platform_fee_withdrawn,token_price,hard_cap,soft_cap,max_per_user,start_time,end_time,tge_time,total_raised,participant_count,status,created_at").
		Model(&entity.Project{}).
		Where("project_id in ?", projectIDs).Scan(&infos).Error
	if err != nil {
		return nil
	}
	res := make(map[int]entity.Project, len(infos))
	for _, info := range infos {
		res[info.ProjectID] = info
	}
	return res
}

func Info(projectID int64) *entity.Project {
	info := entity.Project{}
	err := db.Mysql().
		Select("id,project_id,owner_address,name_en,name_cn,tagline,description,tags,team_info,roadmap,partners,token_address,token_symbol,token_decimals,payment_token,payment_token_address,payment_token_decimals,chain_id,contract_address,contract_version,onchain_locked,fee_receiver,success_fee_rate,platform_fee_withdrawn,token_price,hard_cap,soft_cap,max_per_user,start_time,end_time,tge_time,total_raised,participant_count,status,created_at").
		Model(&entity.Project{}).
		Where("project_id = ?", projectID).Scan(&info).Error
	if err != nil {
		return nil
	}
	return &info
}
