package job

import (
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/job/chainevent"
	"testing"
	"time"
)

func TestV3OnlyRegistersOnlyV3Lanes(t *testing.T) {
	if log.GetLogger() == nil {
		log.Init(t.TempDir(), "ERROR", 1)
	}
	viper.Reset()
	defer viper.Reset()
	viper.Set("application.jobMode", "ido-v3")
	oldCron, oldSource, oldStop := crontab, approvedV3Source, scannerStop
	defer func() { crontab = oldCron; approvedV3Source = oldSource; scannerStop = oldStop }()
	crontab = cron.New(cron.WithSeconds())
	approvedV3Source = &chainevent.ApprovedV3ScannerJob{Stream: chainevent.V3ProxyStreamConfig{Mode: "stream", ExternalProducer: true}}
	starter{}.Chain()
	if len(crontab.Entries()) != 4 {
		t.Fatalf("legacy jobs registered: %d", len(crontab.Entries()))
	}
	if scannerStop != nil {
		scannerStop()
	}
	immediateScanner.Wait()
}

func TestStopAllWaitsForImmediateScanner(t *testing.T) {
	immediateScanner.Add(1)
	stopped := StopAll()
	select {
	case <-stopped.Done():
		immediateScanner.Done()
		t.Fatal("shutdown closed clients while immediate scanner still ran")
	case <-time.After(20 * time.Millisecond):
	}
	immediateScanner.Done()
	select {
	case <-stopped.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after immediate scanner exited")
	}
}

func TestValidateRuntimeMode(t *testing.T) {
	for _, tc := range []struct {
		name, env, mode, v1 string
		scanner, exclusive  bool
		valid               bool
	}{
		{"ido", "prod", "ido", "false", false, false, true},
		{"V3 only", "prod", "ido-v3", "false", false, false, true},
		{"V3 cannot mix", "prod", "ido-v3", "false", true, true, false},
		{"V3 requires V1 off", "prod", "ido-v3", "true", false, false, false},
		{"implicit legacy v1", "prod", "ido", "", false, false, false},
		{"explicit legacy v1", "prod", "ido", "true", false, false, false},
		{"fair-launch without CE-V2", "prod", "fair-launch", "", true, true, false},
		{"implicit prod", "prod", "", "", false, false, false},
		{"mixed prod", "prod", "ido", "false", true, true, false},
		{"scanner without fence", "prod", "fair-launch", "", true, false, false},
		{"disabled scanner", "prod", "fair-launch", "", false, true, false},
		{"unknown role", "prod", "both", "", true, true, false},
		{"legacy local", "local", "", "", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRuntimeMode(tc.env, tc.mode, tc.v1, tc.scanner, tc.exclusive)
			if (err == nil) != tc.valid {
				t.Fatalf("admission err=%v valid=%t", err, tc.valid)
			}
		})
	}
}
