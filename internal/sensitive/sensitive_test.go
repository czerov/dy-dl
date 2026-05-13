package sensitive

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	input := "sessionid=secret; ttwid=secret2; plain=value"
	got := Redact(input)
	if strings.Contains(got, "secret") {
		t.Fatalf("secret was not redacted: %s", got)
	}
	if !strings.Contains(got, "plain=value") {
		t.Fatalf("non-sensitive value should remain: %s", got)
	}
}
