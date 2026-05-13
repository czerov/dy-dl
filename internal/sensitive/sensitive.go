package sensitive

import "regexp"

var cookiePairPattern = regexp.MustCompile(`(?i)\b([a-z0-9_.%-]*(?:session|sid|uid|token|ticket|cookie|csrf|passport|ttwid|odin|web_id|device_id|install_id|verifyfp|nonce|signature|auth)[a-z0-9_.%-]*)=([^;\s]+)`)

func Redact(value string) string {
	return cookiePairPattern.ReplaceAllString(value, "$1=<redacted>")
}
