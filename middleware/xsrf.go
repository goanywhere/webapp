package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/goanywhere/crypto"
)

const (
	xsrfCookieName = "xsrf"
	xsrfHeaderName = "X-XSRF-Token"
	xsrfFieldName  = "xsrftoken"

	xsrfMaxAge  = 3600 * 24 * 365
	xsrfTimeout = time.Hour * 24 * 365
)

var (
	errXSRFReferer = "Referer URL is missing from the request or the value was malformed."
	errXSRFToken   = "Invalid XSRF tokens"

	xsrfPattern   = regexp.MustCompile("[^0-9a-zA-Z-_]")
	unsafeMethods = regexp.MustCompile("^(DELETE|POST|PUT)$")
)

type xsrf struct {
	*http.Request
	http.ResponseWriter
	token string
}

// See http://en.wikipedia.org/wiki/Same-origin_policy
func (self *xsrf) checkOrigin() bool {
	if self.Request.URL.Scheme == "https" {
		// See [OWASP]; Checking the Referer Header.
		referer, err := url.Parse(self.Request.Header.Get("Referer"))

		if err != nil || referer.String() == "" ||
			referer.Scheme != self.Request.URL.Scheme ||
			referer.Host != self.Request.URL.Host {

			return false
		}
	}
	return true
}

func (self *xsrf) checkToken(token string) bool {
	// Header always takes precedance of form field since some popular
	// JavaScript frameworks allow global custom headers for all AJAX requests.
	query := self.Request.Header.Get(xsrfFieldName)
	if query == "" {
		query = self.Request.FormValue(xsrfFieldName)
	}

	// 1) basic length comparison.
	if query == "" || len(query) != len(token) {
		return false
	}
	// *sanitize* incoming masked token.
	query = xsrfPattern.ReplaceAllString(query, "")

	// 2) byte-based comparison.
	a, _ := base64.URLEncoding.DecodeString(token)
	b, _ := base64.URLEncoding.DecodeString(query)
	if subtle.ConstantTimeCompare(a, b) != 1 {
		return false
	}

	// 3) issued time checking.
	index := bytes.LastIndex(b, []byte{'|'})
	if index != 40 {
		return false
	}

	nanos, err := strconv.ParseInt(string(b[index+1:]), 10, 64)
	if err != nil {
		return false
	}
	now := time.Now()
	issueTime := time.Unix(0, nanos)

	if now.Sub(issueTime) >= xsrfTimeout {
		return false
	}

	// Ensure the token is not from the *future*, allow 1 minute grace period.
	if issueTime.After(now.Add(1 * time.Minute)) {
		return false
	}

	return true
}

func (self *xsrf) generate() {
	// Ensure we have XSRF token in the cookie first.
	var token string
	if cookie, err := self.Request.Cookie(xsrfCookieName); err == nil {
		if cookie.Value != "" {
			token = cookie.Value
		}
	}
	if token == "" {
		// Generate a base64-encoded token.
		nano := time.Now().UnixNano()
		hash := hmac.New(sha1.New, []byte(crypto.Random(32)))
		fmt.Fprintf(hash, "%s|%d", crypto.Random(12), nano)
		raw := fmt.Sprintf("%s|%d", hex.EncodeToString(hash.Sum(nil)), nano)
		token = base64.URLEncoding.EncodeToString([]byte(raw))

		// The max-age directive takes priority over Expires.
		//	http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
		cookie := new(http.Cookie)
		cookie.Name = xsrfCookieName
		cookie.Value = token
		cookie.MaxAge = xsrfMaxAge
		cookie.Path = "/"
		cookie.HttpOnly = true
		http.SetCookie(self.ResponseWriter, cookie)
	}
	self.ResponseWriter.Header()[xsrfHeaderName] = []string{token}
	self.token = token
}

// XSRF serves as Cross-Site Request Forgery protection middleware.
func XSRF(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		x := new(xsrf)
		x.Request = r
		x.ResponseWriter = w
		x.generate()

		if unsafeMethods.MatchString(r.Method) {
			// Ensure the URL came for "Referer" under HTTPS.
			if !x.checkOrigin() {
				http.Error(w, errXSRFReferer, http.StatusForbidden)
			}

			// length => bytes => issue time checkpoints.
			if !x.checkToken(x.token) {
				http.Error(w, errXSRFToken, http.StatusForbidden)
			}
		}

		// ensure browser will invalidate the cached XSRF token.
		w.Header().Add("Vary", "Cookie")

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
