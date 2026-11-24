package middleware

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestAuditCredentialBodiesAreOmitted(t *testing.T) {
	body := []byte(`{"old_password":"old-secret","new_password":"new-secret"}`)
	for _, path := range []string{"/admin/v1/self/password", "/admin/v1/admin-users/7/password"} {
		got := auditRequestBody(path, body)
		if strings.Contains(got, "old-secret") || strings.Contains(got, "new-secret") || !strings.Contains(got, "omitted") {
			t.Fatalf("credential body path=%s was not omitted: %s", path, got)
		}
	}
}

func TestAuditRedactsNestedSensitiveFields(t *testing.T) {
	input := []byte(`{"safe":"visible","nested":{"old_password":"old","apiSecret":"hidden","access-token":"bearer"},"items":[{"private_key":"key"}]}`)
	got := auditRequestBody("/admin/v1/example", input)
	for _, forbidden := range []string{`:"old"`, `:"hidden"`, `:"bearer"`, `:"key"`} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sensitive value %q remained in %s", forbidden, got)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["safe"] != "visible" {
		t.Fatalf("safe field changed: %#v", decoded)
	}
}

func TestAuditOmitsNonJSONBodies(t *testing.T) {
	got := auditRequestBody("/admin/v1/upload", []byte("private multipart bytes"))
	if strings.Contains(got, "private multipart bytes") || !strings.Contains(got, "non-JSON") {
		t.Fatalf("non-JSON body was retained: %s", got)
	}
}
