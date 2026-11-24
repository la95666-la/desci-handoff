package utils

import (
	"fmt"
	"time"

	"github.com/sony/sonyflake"
)

const BucketSize = int64(1000 * 60 * 60 * 24 * 10)

var (
	sonyFlake *sonyflake.Sonyflake
	epoch, _  = time.Parse("2006-01-02", "2026-01-01")
)

func SonyFlakeInit(machineId uint16) {
	settings := sonyflake.Settings{
		StartTime: epoch,
		MachineID: func() (uint16, error) {
			return machineId, nil
		},
		CheckMachineID: func(machineID uint16) bool {
			return true
		},
	}

	sonyFlake = sonyflake.NewSonyflake(settings)
	fmt.Println("SonyFlake init successfully")
}

func SonyFlakeId() int64 {
	id, _ := sonyFlake.NextID()
	return int64(id)
}

func SonyFlakeBucketFromId(id *int64) int64 {
	var gap int64
	if id == nil {
		gap = time.Since(epoch).Milliseconds() / 10 * 10
	} else {
		gap = *id >> 24 * 10
	}

	return gap/BucketSize + 1
}

func SonyFlakeBucketFromMicro(micro *int64) int64 {

	var gap int64
	if micro == nil {

		gap = time.Since(epoch).Milliseconds() / 10 * 10
	} else {

		gap = (*micro - epoch.UnixMicro()) / 10000 * 10
	}

	return gap/BucketSize + 1
}

func SonyFlakeBucketsFromMicro(startTime int64, endTime *int64) (int64, int64) {
	return SonyFlakeBucketFromMicro(&startTime), SonyFlakeBucketFromMicro(endTime)
}
