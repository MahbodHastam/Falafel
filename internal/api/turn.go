package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strconv"
	"time"
)

// turnCredentials returns time-limited TURN credentials in the format coturn
// expects with `use-auth-secret`: the username is the expiry timestamp and the
// credential is base64(HMAC-SHA1(secret, username)).
func turnCredentials(secret string, expires time.Time) (username, credential string) {
	username = strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	return username, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
