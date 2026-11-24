package controller

import (
	"encoding/json"
	"testing"

	"joincare_server/pkg/common/entity"
)

func stringPointer(value string) *string { return &value }

func TestNormalizeAlertFrequency(t *testing.T) {
	tests := map[string]string{
		"once":    "once",
		"daily":   "daily",
		"session": "session",
		"invalid": "once",
		"":        "once",
	}
	for input, want := range tests {
		if got := normalizeAlertFrequency(input); got != want {
			t.Fatalf("normalizeAlertFrequency(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeAlertPages(t *testing.T) {
	if got := normalizeAlertPages(" launchpad,home,launchpad,unknown "); got != "launchpad,home" {
		t.Fatalf("unexpected pages: %q", got)
	}
	if got := normalizeAlertPages(""); got != "home,ecosystem,launchpad" {
		t.Fatalf("unexpected default pages: %q", got)
	}
}

func TestAlertBounds(t *testing.T) {
	if got := clampAlertDelay(-1); got != 0 {
		t.Fatalf("negative delay = %d", got)
	}
	if got := clampAlertDelay(99); got != 15 {
		t.Fatalf("large delay = %d", got)
	}
	if got := normalizeAlertRevision(0); got != 1 {
		t.Fatalf("revision = %d", got)
	}
}

func TestParseAlertSchedule(t *testing.T) {
	start, end, err := parseAlertSchedule(
		stringPointer("2026-07-23T10:00:00+08:00"),
		stringPointer("2026-07-24T10:00:00+08:00"),
	)
	if err != nil || start == nil || end == nil {
		t.Fatalf("valid schedule rejected: start=%v end=%v err=%v", start, end, err)
	}
	if _, _, err := parseAlertSchedule(
		stringPointer("2026-07-24T10:00:00+08:00"),
		stringPointer("2026-07-23T10:00:00+08:00"),
	); err == nil {
		t.Fatal("reversed schedule should be rejected")
	}
	if _, _, err := parseAlertSchedule(stringPointer("not-a-date"), nil); err == nil {
		t.Fatal("invalid start should be rejected")
	}
}

func TestImportantNewsAlertJSONShape(t *testing.T) {
	payload := importantNewsAlert{
		PublicNewsDetailResponse: PublicNewsDetailResponse{ID: 7, Title: "Funding news"},
		Frequency:                "once",
		Pages:                    []string{"home"},
		Revision:                 2,
		TargetURL:                "/news/7",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "title", "frequency", "pages", "revision", "target_url"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing flattened JSON field %q: %s", key, data)
		}
	}
}

func TestPickNewsTranslationPrefersRequestedLanguage(t *testing.T) {
	rows := []entity.NewsTranslation{
		{Language: "en", Title: "English title"},
		{Language: "tw", Title: "繁體標題"},
	}
	if got := pickNewsTranslation(rows, "tw"); got.Title != "繁體標題" {
		t.Fatalf("tw title = %q, want 繁體標題", got.Title)
	}
	if got := pickNewsTranslation(rows, "en"); got.Title != "English title" {
		t.Fatalf("en title = %q, want English title", got.Title)
	}
}

func TestPickNewsTranslationFallsBackToEnglish(t *testing.T) {
	rows := []entity.NewsTranslation{
		{Language: "en", Title: "English fallback"},
	}
	got := pickNewsTranslation(rows, "tw")
	if got.Language != "en" || got.Title != "English fallback" {
		t.Fatalf("fallback = %#v, want English translation", got)
	}
}
