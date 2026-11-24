package project

import (
	"slices"
	"testing"

	"joincare_server/pkg/common/constant"
)

func TestPublicProjectStatusesExposeSetupAndSetButExcludeNonPublicStates(t *testing.T) {
	statuses := publicProjectStatuses()

	for _, visible := range []string{constant.ProjectStatusSetup, constant.ProjectStatusSet, constant.ProjectStatusSuccessfulPendingRelease, "V3ReorgHalted"} {
		if !slices.Contains(statuses, visible) {
			t.Fatalf("%s projects must be returned to the public homepage and Launchpad", visible)
		}
	}
	for _, hidden := range []string{"candidate", "archived"} {
		if slices.Contains(statuses, hidden) {
			t.Fatalf("draft status %q must not be publicly listed", hidden)
		}
	}
}

func TestPublicRequestedProjectStatusesFailsClosed(t *testing.T) {
	for _, visible := range append(publicProjectStatuses(), "candidate") {
		if visible == constant.ProjectStatusEnded {
			continue
		}
		statuses, ok := publicRequestedProjectStatuses(visible)
		if !ok || len(statuses) != 1 || statuses[0] != visible {
			t.Fatalf("expected explicit public status %q to be admitted, got %v %v", visible, statuses, ok)
		}
	}
	ended, ok := publicRequestedProjectStatuses(constant.ProjectStatusEnded)
	if !ok || !slices.Equal(ended, []string{constant.ProjectStatusSuccessful, constant.ProjectStatusFailed, constant.ProjectStatusCanceled}) {
		t.Fatalf("ended projection mismatch: %v %v", ended, ok)
	}
	for _, hidden := range []string{"archived", "draft", "", "candidate "} {
		if statuses, ok := publicRequestedProjectStatuses(hidden); ok || statuses != nil {
			t.Fatalf("non-public status %q admitted: %v %v", hidden, statuses, ok)
		}
	}
}

func TestPublicProjectDetailStatusFailsClosed(t *testing.T) {
	for _, visible := range append(publicProjectStatuses(), "candidate") {
		if !publicProjectDetailStatus(visible) {
			t.Fatalf("public status %q rejected", visible)
		}
	}
	for _, hidden := range []string{"archived", "draft", "", "candidate "} {
		if publicProjectDetailStatus(hidden) {
			t.Fatalf("non-public detail status %q admitted", hidden)
		}
	}
}
