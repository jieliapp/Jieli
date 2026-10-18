package redact_test

import (
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/common/redact"
)

func TestRedactTextShouldHandleUserinfoURLWithMalformedPort(t *testing.T) {
	redacted := redact.RedactText("connect https://user:secret@example.com:notaport/path?token=query-secret")

	if strings.Contains(redacted, "user:secret") || strings.Contains(redacted, "query-secret") {
		t.Fatalf("expected URL secrets redacted, got %q", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED:url-userinfo]@example.com:notaport") {
		t.Fatalf("expected malformed-port URL host preserved, got %q", redacted)
	}
	if !strings.Contains(redacted, "token=[REDACTED:url-query-token]") {
		t.Fatalf("expected query token marker, got %q", redacted)
	}
}

func TestRedactTextShouldMaskKnownSecretFormats(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		marker string
	}{
		{"private key", "-----BEGIN PRIVATE KEY-----\nabc123\n-----END PRIVATE KEY-----", "[REDACTED:private-key]"},
		{"authorization bearer", "Authorization: Bearer abc.def.ghi", "Authorization: Bearer [REDACTED:authorization-bearer]"},
		{"aws access key", "AKIA1234567890ABCDEF", "[REDACTED:aws-access-key]"},
		{"github token", "ghp_1234567890abcdefghijklmnopqrstuvwxyz", "[REDACTED:github-token]"},
		{"google api key", "AIza1234567890abcdefghijklmnopqrstuvwxy", "[REDACTED:google-api-key]"},
		{"jwt token", "eyJaaaaaaaaaaa.eyJbbbbbbbbbbb.cccccccccccccc", "[REDACTED:jwt-token]"},
		{"assignment", "API_KEY=super-secret-value", "API_KEY=[REDACTED:api-key]"},
		{"redis cli password", "redis-cli -a hunter2 ping", "redis-cli -a [REDACTED:redis-cli-password] ping"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			redacted := redact.RedactText(tc.input)
			if redacted == tc.input || !strings.Contains(redacted, tc.marker) {
				t.Fatalf("expected marker %q in %q", tc.marker, redacted)
			}
		})
	}
}

func TestRedactJSONShouldPreserveExistingTypedRedactionMarkers(t *testing.T) {
	redacted := string(redact.RedactJSON([]byte(`{"api_key":"[REDACTED:api-key]","content":"token=[REDACTED:url-query-token]"}`)))

	if strings.Count(redacted, "[REDACTED:") != 2 {
		t.Fatalf("expected existing markers to be preserved, got %s", redacted)
	}
	if !strings.Contains(redacted, `"api_key":"[REDACTED:api-key]"`) || !strings.Contains(redacted, `"content":"token=[REDACTED:url-query-token]"`) {
		t.Fatalf("expected existing typed redaction markers unchanged, got %s", redacted)
	}
}

func TestRedactJSONShouldMaskSensitiveKeysAndSkipImagePayloads(t *testing.T) {
	redacted := string(redact.RedactJSON([]byte(`{
		"api_key":"sk-proj-jsonSECRET12345678901234567890",
		"blocks":[
			{"type":"base64","data":"sk-proj-imageSECRET12345678901234567890"},
			{"isImage":true,"content":"sk-proj-contentSECRET12345678901234567890"},
			{"type":"text","text":"jieli_abcdefghijklmnopqrstuvwxyz1234567890"}
		]
	}`)))

	if strings.Contains(redacted, "sk-proj-jsonSECRET") {
		t.Fatalf("expected sensitive key value to be replaced, got %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED:api-key]") {
		t.Fatalf("expected typed marker for sensitive key, got %s", redacted)
	}
	if !strings.Contains(redacted, "sk-proj-imageSECRET") || !strings.Contains(redacted, "sk-proj-contentSECRET") {
		t.Fatalf("expected image/base64 payloads to be preserved, got %s", redacted)
	}
	if strings.Contains(redacted, "jieli_abcdefghijklmnopqrstuvwxyz") || !strings.Contains(redacted, "[REDACTED:jieli-api-key]") {
		t.Fatalf("expected nested text redacted, got %s", redacted)
	}
}
