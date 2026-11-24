package model

import "github.com/robfig/cron/v3"

type Add interface {
	Add(*cron.Cron)
	Run()
}
