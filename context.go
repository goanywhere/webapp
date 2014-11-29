/* ----------------------------------------------------------------------
 *       ______      ___                         __
 *      / ____/___  /   |  ____  __  ___      __/ /_  ___  ________
 *     / / __/ __ \/ /| | / __ \/ / / / | /| / / __ \/ _ \/ ___/ _ \
 *    / /_/ / /_/ / ___ |/ / / / /_/ /| |/ |/ / / / /  __/ /  /  __/
 *    \____/\____/_/  |_/_/ /_/\__. / |__/|__/_/ /_/\___/_/   \___/
 *                            /____/
 *
 * (C) Copyright 2014 GoAnywhere (http://goanywhere.io).
 * ----------------------------------------------------------------------
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 * ----------------------------------------------------------------------*/

package web

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/goanywhere/env"
	"github.com/goanywhere/web/crypto"
)

const ContentType = "Content-Type"

var (
	contextId uint64
	prefix    string
	signature *crypto.Signature
)

type Context struct {
	http.ResponseWriter
	Request *http.Request

	status int
	size   int
	data   map[string]interface{}
}

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	ctx := new(Context)
	ctx.size = -1
	ctx.createSignature()
	ctx.data = make(map[string]interface{})

	ctx.ResponseWriter = w
	ctx.Request = r
	return ctx
}

// createSignature creates a signature for accessing securecookie.
func (self *Context) createSignature() {
	if signature == nil {
		secret := env.Get("secret")
		if secret == "" {
			log.Print("Secret key missing, using a random string now, previous cookie will be invalidate")
			pool := []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*(-_+)")
			secret = crypto.RandomString(64, pool)
		}
		signature = crypto.NewSignature(secret)
	}
}

// Status returns current status code of the Context.
func (self *Context) Status() int {
	return self.status
}

// Size returns the Context size to be responsed.
func (self *Context) Size() int {
	return self.size
}

func (self *Context) Written() bool {
	return self.status != 0
}

// WriteHeader: Implementation of http.ResponseWriter#WriteHeader
func (self *Context) WriteHeader(status int) {
	if status >= 100 && status < 512 {
		self.status = status
		self.ResponseWriter.WriteHeader(status)
	}
}

// Write: Implementation of http.ResponseWriter#Write
func (self *Context) Write(data []byte) (size int, err error) {
	if !self.Written() {
		self.WriteHeader(http.StatusOK)
	}
	size, err = self.ResponseWriter.Write(data)
	self.size += size
	return
}

// Hijack: Implementations of http.Hijackeri#Hijack
func (self *Context) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := self.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

// CloseNotify: Implementations of http.CloseNotifier#CloseNotify
func (self *Context) CloseNotify() <-chan bool {
	return self.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// Flush: Implementations of http.Flusher#Flush
func (self *Context) Flush() {
	flusher, ok := self.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

// Id creates a unique fixed-length (40-bits) identity for Context.
func (self *Context) Id() string {
	return fmt.Sprintf("%s-%07d", prefix, atomic.AddUint64(&contextId, 1))
}

// Get fetches context data under the given key.
func (self *Context) Get(key string) interface{} {
	value, exists := self.data[key]
	if !exists {
		return nil
	}
	return value
}

// Set write value under the given key to context data.
func (self *Context) Set(key string, value interface{}) {
	self.data[key] = value
}

// Clear wipes out all existing context data.
func (self *Context) Clear() {
	for key := range self.data {
		delete(self.data, key)
	}
}

// Delete removes context data under the given key.
func (self *Context) Delete(key string) {
	delete(self.data, key)
}

// Cookie returns the cookie value previously set.
func (self *Context) Cookie(key string) (value string) {
	if cookie, err := self.Request.Cookie(key); err == nil {
		if cookie.Value != "" {
			value = cookie.Value
		}
	}
	return
}

// SetCookie writes cookie to ResponseWriter.
func (self *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(self, cookie)
}

// SecureCookie decodes the signed value from cookie.
// Empty string value will be returned if the signature is invalide or expired.
func (self *Context) SecureCookie(key string) (value string) {
	if src := self.Cookie(key); src != "" {
		if bits, err := signature.Decode(key, src); err == nil {
			value = string(bits)
		}
	}
	return
}

// SetSecureCookie replaces the raw value with a signed one & write the cookie into Context.
func (self *Context) SetSecureCookie(cookie *http.Cookie) {
	if cookie.Value != "" {
		if value, err := signature.Encode(cookie.Name, []byte(cookie.Value)); err == nil {
			cookie.Value = value
		}
	}
	http.SetCookie(self, cookie)
}

// IsAjax checks if the incoming request is AJAX request.
func (self *Context) IsAjax() bool {
	return self.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// Query returns the URL query values.
func (self *Context) Query() url.Values {
	return self.Request.URL.Query()
}

// HTML renders cached HTML templates to response.
func (self *Context) HTML(filename string) {
	self.Header().Set(ContentType, "text/html; charset=utf-8")
	buffer := new(bytes.Buffer)
	//page := template.Parse(filename)
	//page.Execute(buffer, self.data)
	buffer.WriteTo(self)
}

// JSON renders JSON data to response.
func (self *Context) JSON(values map[string]interface{}) {
	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(values)

	if err != nil {
		http.Error(self, err.Error(), http.StatusInternalServerError)
		return
	}

	self.Header().Set(ContentType, "application/json; charset=utf-8")
	self.Write(data)
}

// XML renders XML data to response.
func (self *Context) XML(values interface{}) {
	self.Header().Set(ContentType, "application/xml; charset=utf-8")
	encoder := xml.NewEncoder(self)
	encoder.Encode(values)
}

// String writes plain text back to the HTTP response.
func (self *Context) String(format string, values ...interface{}) {
	self.Header().Set(ContentType, "text/plain; charset=utf-8")
	self.Write([]byte(fmt.Sprintf(format, values...)))
}

// Data writes binary data back into the HTTP response.
func (self *Context) Data(data []byte) {
	self.Header().Set(ContentType, "application/octet-stream")
	self.Write(data)
}

func init() {
	// Here we generate a md5-based fixed length (32-bits) prefix for ContextId.
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	// system pid combined with timestamp to identity current go process.
	pid := fmt.Sprintf("%d:%d", os.Getpid(), time.Now().UnixNano())
	hash := hmac.New(md5.New, []byte(fmt.Sprintf("%s-%s", hostname, pid)))
	prefix = hex.EncodeToString(hash.Sum(nil))
}
