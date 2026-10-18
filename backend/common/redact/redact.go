package redact

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

const markerPrefix = "[REDACTED:"

type textRule struct {
	id                    string
	pattern               *regexp.Regexp
	keywords              []string
	caseSensitiveKeywords bool
	replacement           func(textRule, string) string
}

var (
	invisibleTagCharsRE = regexp.MustCompile(`[\x{E0000}-\x{E007F}]`)
	urlRE               = regexp.MustCompile(`(?i)\b(?:https?|wss?|ftp|file|redis|rediss|mongodb(?:\+srv)?|postgres(?:ql)?|mysql|mariadb)://[^\s<>"')]+`)
	sensitiveKeyRE      = regexp.MustCompile(`(?i)(?:^|[_\-.])(?:api[_\-.]?key|api[_\-.]?token|access[_\-.]?token|auth[_\-.]?token|ws[_\-.]?token|rvt[_\-.]?token|token|secret|client[_\-.]?secret|password|passwd|pwd|jwt|sessionid|session|sid|authorization|bearer|rediscli[_\-.]?auth|database[_\-.]?url|redis[_\-.]?url|mongo(?:db)?[_\-.]?(?:uri|url)|connection[_\-.]?string|private[_\-.]?key)(?:$|[_\-.])`)
	querySeparatorRE    = regexp.MustCompile(`([&;])`)
	compactKeyRE        = regexp.MustCompile(`[^a-z0-9]`)
	sensitiveIDRE       = regexp.MustCompile(`[^a-z0-9]+`)
)

var sensitiveQueryKeywords = []string{
	"token",
	"key",
	"api_key",
	"apikey",
	"access_token",
	"secret",
	"password",
	"auth",
	"authorization",
	"bearer",
	"jwt",
	"session",
	"sessionid",
	"sid",
}

var secretRules = []textRule{
	{
		id:       "private-key",
		pattern:  regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
		keywords: []string{"private key", "begin"},
	},
	{
		id:          "authorization-bearer",
		pattern:     regexp.MustCompile(`(?i)(Authorization\s*:\s*Bearer\s+)[^\s"']+`),
		keywords:    []string{"authorization", "bearer"},
		replacement: replaceAuthorizationBearer,
	},
	{
		id:       "openai-api-key",
		pattern:  regexp.MustCompile(`(?i)\b(?:sk-ant-[A-Za-z0-9_-]+|sk-(?:proj|live|test)-[A-Za-z0-9_-]+|sk-[A-Za-z0-9_-]{20,})\b`),
		keywords: []string{"sk-", "sk-ant"},
	},
	{
		id:                    "aws-access-key",
		pattern:               regexp.MustCompile(`\b(?:AKIA|ASIA|A3T)[A-Z0-9]{16}\b`),
		keywords:              []string{"AKIA", "ASIA", "A3T"},
		caseSensitiveKeywords: true,
	},
	{
		id:       "github-token",
		pattern:  regexp.MustCompile(`\b(?:ghp_[0-9A-Za-z]{36}|gho_[0-9A-Za-z]{36}|(?:ghu|ghs)_[0-9A-Za-z]{36}|ghr_[0-9A-Za-z]{76}|github_pat_[A-Za-z0-9]{22}_[A-Za-z0-9]{59}|gh[ps]_[A-Za-z0-9_]{20,})\b`),
		keywords: []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"},
	},
	{id: "npm-token", pattern: regexp.MustCompile(`\bnpm_[A-Za-z0-9]{20,}\b`), keywords: []string{"npm_"}},
	{id: "gitlab-token", pattern: regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`), keywords: []string{"glpat-"}},
	{id: "bitbucket-token", pattern: regexp.MustCompile(`\bb(?:bpat|brat)-[A-Za-z0-9_-]{20,}\b`), keywords: []string{"bbpat-", "bbrat-"}},
	{id: "huggingface-token", pattern: regexp.MustCompile(`\bhf_[A-Za-z0-9]{20,}\b`), keywords: []string{"hf_"}},
	{id: "slack-token", pattern: regexp.MustCompile(`\bx(?:ox[abprs]|app|wfp)-[A-Za-z0-9-]{10,}\b`), keywords: []string{"xox", "xapp-", "xwfp-"}},
	{id: "slack-webhook", pattern: regexp.MustCompile(`https://hooks\.slack\.com/(?:services|triggers|workflows)/[A-Za-z0-9/_-]+`), keywords: []string{"hooks.slack.com"}},
	{id: "stripe-secret-key", pattern: regexp.MustCompile(`\bsk_(?:test|live)_[A-Za-z0-9]{16,}\b`), keywords: []string{"sk_test_", "sk_live_"}},
	{id: "supabase-token", pattern: regexp.MustCompile(`\bsbp_[A-Za-z0-9_-]{20,}\b`), keywords: []string{"sbp_"}},
	{id: "pypi-token", pattern: regexp.MustCompile(`\bpypi-[A-Za-z0-9_-]{20,}\b`), keywords: []string{"pypi-"}},
	{id: "cloudflare-token", pattern: regexp.MustCompile(`\bcfut_[A-Za-z0-9_-]{20,}\b`), keywords: []string{"cfut_"}},
	{id: "e2b-token", pattern: regexp.MustCompile(`\be2b_[A-Za-z0-9_-]{20,}\b`), keywords: []string{"e2b_"}},
	{id: "jieli-api-key", pattern: regexp.MustCompile(`\bjieli_[A-Za-z0-9_-]{20,}\b`), keywords: []string{"jieli_"}},
	{id: "google-api-key", pattern: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`), keywords: []string{"AIza"}, caseSensitiveKeywords: true},
	{id: "sendgrid-token", pattern: regexp.MustCompile(`\bSG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\b`), keywords: []string{"SG."}, caseSensitiveKeywords: true},
	{id: "linear-token", pattern: regexp.MustCompile(`\blin_api_[A-Za-z0-9_-]{20,}\b`), keywords: []string{"lin_api_"}},
	{id: "postman-token", pattern: regexp.MustCompile(`\bPMAK-[A-Za-z0-9-]{20,}\b`), keywords: []string{"PMAK-"}, caseSensitiveKeywords: true},
	{id: "pulumi-token", pattern: regexp.MustCompile(`\bpul-[A-Za-z0-9_-]{20,}\b`), keywords: []string{"pul-"}},
	{id: "databricks-token", pattern: regexp.MustCompile(`\bdapi[A-Za-z0-9]{20,}\b`), keywords: []string{"dapi"}},
	{id: "grafana-token", pattern: regexp.MustCompile(`\beyJrIjoi[A-Za-z0-9_-]{20,}\b`), keywords: []string{"eyJrIjoi"}, caseSensitiveKeywords: true},
	{id: "mapbox-token", pattern: regexp.MustCompile(`\bpk\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), keywords: []string{"pk."}},
	{id: "jwt-token", pattern: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), keywords: []string{"eyJ"}, caseSensitiveKeywords: true},
	{id: "age-secret-key", pattern: regexp.MustCompile(`\bAGE-SECRET-KEY-1[A-Z0-9]{20,}\b`), keywords: []string{"AGE-SECRET-KEY-1"}, caseSensitiveKeywords: true},
	{
		id:          "sensitive-assignment",
		pattern:     regexp.MustCompile(`(?i)\b([A-Z0-9_.-]*(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD|PASSWD|PWD|REDISCLI_AUTH)[A-Z0-9_.-]*\s*(?:=|:=|:)\s*)(["']?)([^\s,"']+)(["']?)`),
		keywords:    []string{"api", "key", "token", "secret", "password", "passwd", "pwd", "rediscli_auth"},
		replacement: replaceSensitiveAssignment,
	},
	{
		id:          "json-yaml-sensitive-key",
		pattern:     regexp.MustCompile(`(?i)(["']?[A-Za-z0-9_.-]*(?:api[_-]?key|api[_-]?token|access[_-]?token|auth[_-]?token|ws[_-]?token|rvt[_-]?token|token|secret|client[_-]?secret|password|passwd|pwd|jwt|sessionid|session|sid)[A-Za-z0-9_.-]*["']?\s*(?:=|:=|:)\s*)(["']?)([^\s,\n\r"'}\]]+)(["']?)`),
		keywords:    []string{"api", "token", "secret", "password", "session", "jwt"},
		replacement: replaceSensitiveAssignment,
	},
	{
		id:          "redis-cli-password",
		pattern:     regexp.MustCompile(`(?i)\b(redis-cli|redis-server)(\s+(?:-a|--pass|--requirepass)\s+)([^\s"']+)`),
		keywords:    []string{"redis-cli", "redis-server", "--pass", "--requirepass"},
		replacement: replaceRedisPassword,
	},
	{
		id:          "gcp-service-account",
		pattern:     regexp.MustCompile(`(?i)(["']type["']\s*:\s*["'])service_account(["'])`),
		keywords:    []string{"service_account"},
		replacement: replaceGCPServiceAccount,
	},
}

func typedRedaction(ruleID string) string {
	return markerPrefix + ruleID + "]"
}

// RedactText replaces known secret patterns with typed redaction markers.
func RedactText(value string) string {
	redacted := invisibleTagCharsRE.ReplaceAllString(value, "")
	redacted = redactURLs(redacted)
	for _, rule := range secretRules {
		if rule.appliesTo(redacted) {
			redacted = rule.apply(redacted)
		}
	}
	return redacted
}

// RedactJSON recursively redacts string values in a JSON payload. Invalid JSON
// is treated as text so callers never store an unredacted value just because a
// client sent malformed content.
func RedactJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return []byte(RedactText(string(raw)))
	}
	redacted := redactJSONValue(value)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return raw
	}
	return encoded
}

func (rule textRule) appliesTo(value string) bool {
	if len(rule.keywords) == 0 {
		return true
	}
	haystack := value
	if !rule.caseSensitiveKeywords {
		haystack = strings.ToLower(value)
	}
	for _, keyword := range rule.keywords {
		needle := keyword
		if !rule.caseSensitiveKeywords {
			needle = strings.ToLower(keyword)
		}
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func (rule textRule) apply(value string) string {
	if rule.replacement != nil {
		return rule.replacement(rule, value)
	}
	return rule.pattern.ReplaceAllString(value, typedRedaction(rule.id))
}

func replaceAuthorizationBearer(rule textRule, value string) string {
	return rule.pattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := rule.pattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return parts[1] + typedRedaction(rule.id)
	})
}

func replaceSensitiveAssignment(rule textRule, value string) string {
	return rule.pattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := rule.pattern.FindStringSubmatch(match)
		if len(parts) < 5 {
			return match
		}
		if strings.HasPrefix(parts[3], markerPrefix) {
			return match
		}
		return parts[1] + parts[2] + typedRedaction(sensitiveKeyID(parts[1], "")) + parts[4]
	})
}

func replaceRedisPassword(rule textRule, value string) string {
	return rule.pattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := rule.pattern.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		if strings.HasPrefix(parts[3], markerPrefix) {
			return match
		}
		return parts[1] + parts[2] + typedRedaction(rule.id)
	})
}

func replaceGCPServiceAccount(rule textRule, value string) string {
	return rule.pattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := rule.pattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return parts[1] + typedRedaction(rule.id) + parts[2]
	})
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return RedactText(typed)
	case []any:
		for i, item := range typed {
			typed[i] = redactJSONValue(item)
		}
		return typed
	case map[string]any:
		if isBase64Payload(typed) {
			return typed
		}
		for key, item := range typed {
			if isSensitiveJSONKey(key) {
				typed[key] = typedRedaction(sensitiveKeyID(key, ""))
			} else if isImageContentKey(typed, key) {
				typed[key] = item
			} else {
				typed[key] = redactJSONValue(item)
			}
		}
		return typed
	default:
		return value
	}
}

func isSensitiveJSONKey(key string) bool {
	return sensitiveKeyRE.MatchString(key)
}

func isBase64Payload(value map[string]any) bool {
	typeValue, ok := value["type"].(string)
	if !ok {
		return false
	}
	if typeValue != "base64" && typeValue != "image" {
		return false
	}
	_, hasData := value["data"]
	return hasData
}

func isImageContentKey(container map[string]any, key string) bool {
	if key != "content" {
		return false
	}
	isImage, ok := container["isImage"].(bool)
	return ok && isImage
}

func redactURLs(value string) string {
	return urlRE.ReplaceAllStringFunc(value, redactURL)
}

func redactURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return redactMalformedURL(value)
	}
	host := parsed.Host
	if parsed.User != nil {
		host = typedRedaction("url-userinfo") + "@" + host
	}
	query := redactQuery(parsed.RawQuery)
	result := parsed.Scheme + "://" + host + parsed.EscapedPath()
	if parsed.ForceQuery || query != "" {
		result += "?" + query
	}
	return result
}

func redactMalformedURL(value string) string {
	scheme, rest, ok := strings.Cut(value, "://")
	if !ok || scheme == "" {
		return value
	}
	beforeQuery, rawQuery, hasQuery := strings.Cut(rest, "?")
	pathStart := strings.IndexAny(beforeQuery, "/#")
	authority := beforeQuery
	path := ""
	if pathStart >= 0 {
		authority = beforeQuery[:pathStart]
		path = beforeQuery[pathStart:]
	}
	if _, suffix, hasUserinfo := strings.Cut(authority, "@"); hasUserinfo {
		authority = typedRedaction("url-userinfo") + "@" + suffix
	}
	result := scheme + "://" + authority + path
	if hasQuery {
		result += "?" + redactQuery(rawQuery)
	}
	return result
}

func redactQuery(query string) string {
	if query == "" {
		return query
	}
	parts := querySeparatorRE.Split(query, -1)
	separators := querySeparatorRE.FindAllString(query, -1)
	redacted := make([]string, 0, len(parts)+len(separators))
	for i, part := range parts {
		key, _, ok := strings.Cut(part, "=")
		if ok && isSensitiveQueryKey(key) {
			redacted = append(redacted, key+"="+typedRedaction(sensitiveKeyID(key, "url-query")))
		} else {
			redacted = append(redacted, part)
		}
		if i < len(separators) {
			redacted = append(redacted, separators[i])
		}
	}
	return strings.Join(redacted, "")
}

func isSensitiveQueryKey(key string) bool {
	lowered := strings.ToLower(key)
	compact := compactKeyRE.ReplaceAllString(lowered, "")
	for _, keyword := range sensitiveQueryKeywords {
		if strings.Contains(lowered, keyword) || strings.Contains(compact, strings.ReplaceAll(keyword, "_", "")) {
			return true
		}
	}
	return false
}

func sensitiveKeyID(rawKey string, prefix string) string {
	key := sensitiveIDRE.ReplaceAllString(strings.ToLower(rawKey), "-")
	key = strings.Trim(key, "-")
	if key == "" {
		key = "value"
	}
	if prefix == "" {
		return key
	}
	return prefix + "-" + key
}
