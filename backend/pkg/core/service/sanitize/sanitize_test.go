package sanitize

import (
	"strings"
	"testing"
)

func TestRichHTMLRemovesActiveContent(t *testing.T) {
	input := `<h2 onclick="alert(1)">Update</h2><script>alert(1)</script><a href="javascript:alert(1)">bad</a><img src="https://cdn.example/image.png" onerror="alert(1)">`
	got := RichHTML(input)
	for _, forbidden := range []string{"<script", "onclick", "javascript:", "onerror"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Fatalf("sanitized HTML still contains %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "Update") || !strings.Contains(got, "https://cdn.example/image.png") {
		t.Fatalf("safe content was unexpectedly removed: %s", got)
	}
}

func TestPlainTextRemovesMarkup(t *testing.T) {
	if got := PlainText("<b>Release</b><script>x</script>"); got != "Release" {
		t.Fatalf("unexpected plain text: %q", got)
	}
}
