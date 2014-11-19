/**
 *  ------------------------------------------------------------
 *  @project	web.go
 *  @file       utils.go
 *  @date       2014-11-19
 *  @author     Jim Zhan <jim.zhan@me.com>
 *
 *  Copyright © 2014 Jim Zhan.
 *  ------------------------------------------------------------
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
 *  ------------------------------------------------------------
 */
package web

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
)

// Deserialize converts base64-encoded string back to its original object.
func Deserialize(value string, object interface{}) (err error) {
	if bits, err := base64.URLEncoding.DecodeString(value); err == nil {
		err = gob.NewDecoder(bytes.NewBuffer(bits)).Decode(object)
	}
	return
}

// Serialize converts any given object into base64-encoded string using `encoding/gob`.
// NOTE struct must be registered using gob.Register() first.
func Serialize(object interface{}) (value string, err error) {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	if err = encoder.Encode(object); err == nil {
		value = base64.URLEncoding.EncodeToString(buffer.Bytes())
	}
	return
}
