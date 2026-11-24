package entity

import "time"

// FairLaunchEffectApplication is the durable exactly-once guard for every
// project, position, and ledger mutation derived from one canonical event.
type FairLaunchEffectApplication struct {
	EventIdentityID      uint64    `gorm:"column:event_identity_id;primaryKey" json:"event_identity_id"`
	EffectVersion        uint32    `gorm:"column:effect_version;primaryKey" json:"effect_version"`
	ApplicationAttemptID uint64    `gorm:"column:application_attempt_id;not null" json:"application_attempt_id"`
	ObservationID        uint64    `gorm:"column:observation_id;not null" json:"observation_id"`
	EventName            string    `gorm:"column:event_name;size:32;not null" json:"event_name"`
	PayloadDigest        string    `gorm:"column:payload_digest;size:64;not null" json:"payload_digest"`
	EffectDigest         string    `gorm:"column:effect_digest;size:64;not null" json:"effect_digest"`
	AppliedAt            time.Time `gorm:"column:applied_at;not null" json:"applied_at"`
}

func (*FairLaunchEffectApplication) TableName() string {
	return "fair_launch_effect_application"
}
