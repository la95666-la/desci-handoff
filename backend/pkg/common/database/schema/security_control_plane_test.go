package schema

import (
	"strings"
	"testing"
)

func TestSecurityControlPlaneTrustAnchorIsPinned(t *testing.T) {
	if securityControlPlaneMigration != "2026_08_30_security_control_plane_and_event_fence.sql" {
		t.Fatalf("unexpected security migration %q", securityControlPlaneMigration)
	}
	if len(securityControlPlaneChecksum) != 64 {
		t.Fatalf("unexpected security migration checksum %q", securityControlPlaneChecksum)
	}
}

func TestSecurityControlPlaneBooleanMetadataCompatibility(t *testing.T) {
	if !strings.Contains(securityControlPlaneColumnsQuery, "COLUMN_TYPE IN ('tinyint','tinyint(1)')") {
		t.Fatal("security gate must accept MySQL boolean metadata aliases")
	}
	if !strings.Contains(securityControlPlaneColumnsQuery, "IS_NULLABLE='NO' AND COLUMN_DEFAULT='1'") {
		t.Fatal("boolean metadata compatibility must retain NOT NULL DEFAULT 1")
	}
}
