// Command fair-launch-local-e2e is a dedicated LOCAL-ONLY admission probe and
// runtime entry point. It is intentionally not imported by production commands,
// build workflows, release packaging, Dockerfiles, or service definitions.
package main

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"joincare_server/pkg/job/job/chainevent"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: fair-launch-local-e2e admit|serve <frontend-origin> <admin-origin> [127.0.0.1:0]|apply-tx <hash>|wait-apply|wait-apply-replay|wait-apply-next|wait-apply-sequence <2..8>|wait-apply-project-tamper|wait-apply-position-tamper|wait-apply-attestation-tamper|wait-apply-archive-retry (canonical admission bundle on stdin)")
	}
	mode := os.Args[1]
	sequenceCount := 1
	switch mode {
	case "admit", "wait-apply", "wait-apply-replay", "wait-apply-next", "wait-apply-fence-loss", "wait-apply-project-tamper", "wait-apply-position-tamper", "wait-apply-attestation-tamper", "wait-apply-archive-retry":
		if len(os.Args) != 2 {
			fail("command arguments invalid")
		}
	case "apply-tx":
		if len(os.Args) != 3 {
			fail("command arguments invalid")
		}
	case "wait-apply-sequence":
		if len(os.Args) != 3 {
			fail("command arguments invalid")
		}
		parsed, parseErr := strconv.Atoi(os.Args[2])
		if parseErr != nil || parsed < 2 || parsed > 8 {
			fail("wait sequence count invalid")
		}
		sequenceCount = parsed
	case "serve":
		if len(os.Args) < 4 || len(os.Args) > 5 {
			fail("command arguments invalid")
		}
	default:
		fail("command invalid")
	}
	bundle, err := chainevent.DecodeFairLaunchLocalE2EAdmissionBundle(os.Stdin)
	if err != nil {
		fail(err.Error())
	}
	timeout := 30 * time.Second
	if mode == "wait-apply" || mode == "wait-apply-replay" || mode == "wait-apply-next" || mode == "wait-apply-fence-loss" || mode == "wait-apply-sequence" || mode == "wait-apply-project-tamper" || mode == "wait-apply-position-tamper" || mode == "wait-apply-attestation-tamper" || mode == "wait-apply-archive-retry" {
		timeout = 180 * time.Second
	}
	if mode == "wait-apply-sequence" {
		timeout = 225 * time.Second
	}
	rootContext := context.Background()
	if mode == "serve" {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(rootContext, timeout)
	defer cancel()
	capability, database, err := chainevent.AdmitFairLaunchLocalE2E(ctx, bundle, time.Now().UTC())
	if err != nil {
		fail(err.Error())
	}
	ingest, err := chainevent.OpenFairLaunchLocalE2EIngest(ctx, capability, bundle, time.Now().UTC())
	bundle.MySQLJobProfile = ""
	bundle.MySQLIngestProfile = ""
	if err != nil {
		fail(err.Error())
	}
	ingestSQL, err := ingest.DB()
	if err != nil {
		fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
	}
	defer ingestSQL.Close()
	sqlDB, err := database.DB()
	if err != nil {
		fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
	}
	defer sqlDB.Close()
	if mode == "serve" {
		bindAddress := "127.0.0.1:0"
		if len(os.Args) == 5 {
			bindAddress = os.Args[4]
		}
		runtime, runtimeErr := chainevent.NewFairLaunchLocalE2EHTTPServer(capability, ingest, database, chainevent.FairLaunchLocalE2EHTTPOptions{BindAddress: bindAddress, FrontendOrigin: os.Args[2], AdminOrigin: os.Args[3]}, time.Now().UTC())
		if runtimeErr != nil {
			fail(runtimeErr.Error())
		}
		raw, marshalErr := jsonv2.Marshal(runtime.Authority())
		if marshalErr != nil {
			fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
		}
		fmt.Println(string(raw))
		serveContext, stop := signal.NotifyContext(rootContext, os.Interrupt, syscall.SIGTERM)
		defer stop()
		errChannel := make(chan error, 1)
		go func() { errChannel <- runtime.Serve() }()
		select {
		case serveErr := <-errChannel:
			if serveErr != nil {
				fail(serveErr.Error())
			}
		case <-serveContext.Done():
			shutdownContext, shutdownCancel := context.WithTimeout(rootContext, 10*time.Second)
			defer shutdownCancel()
			if shutdownErr := runtime.Shutdown(shutdownContext); shutdownErr != nil {
				fail(shutdownErr.Error())
			}
		}
		return
	}
	if mode == "apply-tx" || mode == "wait-apply" || mode == "wait-apply-replay" || mode == "wait-apply-next" || mode == "wait-apply-fence-loss" || mode == "wait-apply-sequence" || mode == "wait-apply-project-tamper" || mode == "wait-apply-position-tamper" || mode == "wait-apply-attestation-tamper" || mode == "wait-apply-archive-retry" {
		var writes []chainevent.FairLaunchCEWrite
		if mode == "apply-tx" {
			writes, err = chainevent.LoadFairLaunchLocalE2ETransaction(ctx, capability, os.Args[2])
		} else {
			fmt.Fprintln(os.Stderr, "fair-launch-local-e2e: READY")
			admission, admissionErr := capability.Admission()
			if admissionErr != nil {
				fail(admissionErr.Error())
			}
			writes, err = chainevent.WaitFairLaunchLocalE2ETransaction(ctx, capability, admission.SignedTipBlock)
		}
		if err != nil {
			fail("load: " + err.Error())
		}
		if mode == "wait-apply-fence-loss" {
			admission, admissionErr := capability.Admission()
			if admissionErr != nil {
				fail(admissionErr.Error())
			}
			update := database.Exec("UPDATE source_materialization_cursor SET fence_epoch=fence_epoch+1 WHERE source_id=?", admission.SourceID)
			if update.Error != nil || update.RowsAffected != 1 {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
		}
		if mode == "wait-apply-project-tamper" || mode == "wait-apply-archive-retry" {
			if writes[0].Event.ProjectID == nil || !writes[0].Event.ProjectID.IsUint64() {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			update := database.Exec("UPDATE fair_launch_project_state_v3 SET total_raised_raw='1' WHERE project_id=? AND total_raised_raw='0'", writes[0].Event.ProjectID.Uint64())
			if update.Error != nil || update.RowsAffected != 1 {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
		}
		result, err := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, writes[0], time.Now().UTC())
		if err != nil {
			var rejected *chainevent.FairLaunchLocalE2ERejectedEvent
			if errors.As(err, &rejected) {
				if mode == "wait-apply-archive-retry" {
					restore := database.Exec("UPDATE fair_launch_project_state_v3 SET total_raised_raw='0' WHERE project_id=? AND total_raised_raw='1'", writes[0].Event.ProjectID.Uint64())
					if restore.Error != nil || restore.RowsAffected != 1 {
						fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
					}
					recovered, recoveryErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, writes[0], time.Now().UTC())
					if recoveryErr != nil {
						fail("archive retry: " + recoveryErr.Error())
					}
					recoveryEvidence, recoveryErr := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, recovered)
					if recoveryErr != nil {
						fail("archive retry verify: " + recoveryErr.Error())
					}
					var identityCount, placementCount, observationCount int64
					if ingest.Model(&struct{ ID uint64 }{}).Table("chain_event_identity_v2").Where("id=?", recovered.IdentityID).Count(&identityCount).Error != nil || ingest.Model(&struct{ ID uint64 }{}).Table("chain_event_placement_v2").Where("event_identity_id=?", recovered.IdentityID).Count(&placementCount).Error != nil || ingest.Model(&struct{ ID uint64 }{}).Table("chain_event_placement_observation_v2").Where("event_identity_id=?", recovered.IdentityID).Count(&observationCount).Error != nil || identityCount != 1 || placementCount != 1 || observationCount != 1 {
						fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
					}
					raw, marshalErr := jsonv2.Marshal(struct {
						Rejected         *chainevent.FairLaunchLocalE2ERejectedEvent  `json:"rejected"`
						Recovered        chainevent.FairLaunchLocalE2EAppliedEvidence `json:"recovered"`
						IdentityCount    int64                                        `json:"identityCount"`
						PlacementCount   int64                                        `json:"placementCount"`
						ObservationCount int64                                        `json:"observationCount"`
					}{Rejected: rejected, Recovered: recoveryEvidence, IdentityCount: identityCount, PlacementCount: placementCount, ObservationCount: observationCount})
					if marshalErr != nil {
						fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
					}
					fmt.Println(string(raw))
					return
				}
				raw, marshalErr := jsonv2.Marshal(rejected)
				if marshalErr != nil {
					fail("process: " + err.Error())
				}
				fmt.Println(string(raw))
				return
			}
			fail("process: " + err.Error())
		}
		evidence, err := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, result)
		if err != nil {
			fail("verify: " + err.Error())
		}
		if mode == "wait-apply-position-tamper" {
			update := database.Exec("UPDATE fair_launch_position_v3 SET contribution_amount_raw=CONCAT(contribution_amount_raw,'0') WHERE source_application_lineage_id=?", evidence.LineageID)
			if update.Error != nil || update.RowsAffected != 1 {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			_, replayErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, writes[0], time.Now().UTC())
			var rejected *chainevent.FairLaunchLocalE2ERejectedEvent
			if !errors.As(replayErr, &rejected) {
				fail("position tamper replay was not rejected")
			}
			raw, marshalErr := jsonv2.Marshal(struct {
				Forward  chainevent.FairLaunchLocalE2EAppliedEvidence `json:"forward"`
				Rejected *chainevent.FairLaunchLocalE2ERejectedEvent  `json:"rejected"`
			}{Forward: evidence, Rejected: rejected})
			if marshalErr != nil {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			fmt.Println(string(raw))
			return
		}
		if mode == "wait-apply-attestation-tamper" {
			update := database.Exec("UPDATE fair_launch_effect_attestation_v3 SET attestation_digest=REPEAT('0',64) WHERE source_application_lineage_id=?", evidence.LineageID)
			if update.Error == nil {
				fail("attestation tamper was accepted")
			}
			replayResult, replayErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, writes[0], time.Now().UTC())
			if replayErr != nil {
				fail("attestation protected replay: " + replayErr.Error())
			}
			replayEvidence, replayErr := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, replayResult)
			if replayErr != nil {
				fail("attestation protected replay verify: " + replayErr.Error())
			}
			raw, marshalErr := jsonv2.Marshal(struct {
				Forward        chainevent.FairLaunchLocalE2EAppliedEvidence `json:"forward"`
				TamperRejected bool                                         `json:"tamperRejected"`
				Replay         chainevent.FairLaunchLocalE2EAppliedEvidence `json:"replay"`
			}{Forward: evidence, TamperRejected: true, Replay: replayEvidence})
			if marshalErr != nil {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			fmt.Println(string(raw))
			return
		}
		if mode == "wait-apply-replay" {
			replayResult, replayErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, writes[0], time.Now().UTC())
			if replayErr != nil {
				fail("replay: " + replayErr.Error())
			}
			replayEvidence, replayErr := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, replayResult)
			if replayErr != nil {
				fail("replay verify: " + replayErr.Error())
			}
			raw, marshalErr := jsonv2.Marshal(struct {
				Forward chainevent.FairLaunchLocalE2EAppliedEvidence `json:"forward"`
				Replay  chainevent.FairLaunchLocalE2EAppliedEvidence `json:"replay"`
			}{Forward: evidence, Replay: replayEvidence})
			if marshalErr != nil {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			fmt.Println(string(raw))
			return
		}
		if mode == "wait-apply-next" {
			fmt.Fprintln(os.Stderr, "fair-launch-local-e2e: NEXT_READY")
			nextWrites, nextErr := chainevent.WaitFairLaunchLocalE2ETransaction(ctx, capability, writes[0].Event.Log.BlockNumber)
			if nextErr != nil {
				fail("next load: " + nextErr.Error())
			}
			nextResult, nextErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, nextWrites[0], time.Now().UTC())
			if nextErr != nil {
				fail("next process: " + nextErr.Error())
			}
			nextEvidence, nextErr := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, nextResult)
			if nextErr != nil {
				fail("next verify: " + nextErr.Error())
			}
			raw, marshalErr := jsonv2.Marshal(struct {
				First chainevent.FairLaunchLocalE2EAppliedEvidence `json:"first"`
				Next  chainevent.FairLaunchLocalE2EAppliedEvidence `json:"next"`
			}{First: evidence, Next: nextEvidence})
			if marshalErr != nil {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			fmt.Println(string(raw))
			return
		}
		if mode == "wait-apply-sequence" {
			evidenceRows := []chainevent.FairLaunchLocalE2EAppliedEvidence{evidence}
			fromBlock := writes[0].Event.Log.BlockNumber
			for index := 1; index < sequenceCount; index++ {
				fmt.Fprintf(os.Stderr, "fair-launch-local-e2e: STEP_APPLIED %d/%d\n", index, sequenceCount)
				nextWrites, nextErr := chainevent.WaitFairLaunchLocalE2ETransaction(ctx, capability, fromBlock)
				if nextErr != nil {
					fail("sequence load: " + nextErr.Error())
				}
				nextResult, nextErr := chainevent.ProcessFairLaunchLocalE2E(capability, ingest, database, nextWrites[0], time.Now().UTC())
				if nextErr != nil {
					var rejected *chainevent.FairLaunchLocalE2ERejectedEvent
					if errors.As(nextErr, &rejected) {
						raw, marshalErr := jsonv2.Marshal(rejected)
						if marshalErr == nil {
							fmt.Println(string(raw))
							return
						}
					}
					fail("sequence process: " + nextErr.Error())
				}
				nextEvidence, nextErr := chainevent.VerifyFairLaunchLocalE2EApplied(capability, database, nextResult)
				if nextErr != nil {
					fail("sequence verify: " + nextErr.Error())
				}
				evidenceRows = append(evidenceRows, nextEvidence)
				fromBlock = nextWrites[0].Event.Log.BlockNumber
			}
			raw, marshalErr := jsonv2.Marshal(evidenceRows)
			if marshalErr != nil {
				fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
			}
			fmt.Println(string(raw))
			return
		}
		raw, err := jsonv2.Marshal(evidence)
		if err != nil {
			fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
		}
		fmt.Println(string(raw))
		return
	}
	admission, err := capability.Admission()
	if err != nil {
		fail(err.Error())
	}
	raw, err := jsonv2.Marshal(admission)
	if err != nil {
		fail(chainevent.FairLaunchLocalE2EAdmissionRejected)
	}
	fmt.Println(string(raw))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "fair-launch-local-e2e: FAIL:", message)
	os.Exit(1)
}
