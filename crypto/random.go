/* ----------------------------------------------------------------------
 *       ______      ___                         __
 *      / ____/___  /   |  ____  __  ___      __/ /_  ___  ________
 *     / / __/ __ \/ /| | / __ \/ / / / | /| / / __ \/ _ \/ ___/ _ \
 *    / /_/ / /_/ / ___ |/ / / / /_/ /| |/ |/ / / / /  __/ /  /  __/
 *    \____/\____/_/  |_/_/ /_/\__. / |__/|__/_/ /_/\___/_/   \___/
 *                            /____/
 *
 *	(C) Copyright 2014 GoAnywhere (http://goanywhere.io).
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

package crypto

import (
	"math/rand"
	"time"
)

var (
	alphanum = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	random   *rand.Rand
)

// RandomString creates a securely generated random string.
//
//	Args:
//		length: length of the generated random string.
func RandomString(length int, chars []rune) string {
	bytes := make([]rune, length)

	var pool []rune
	if chars == nil {
		pool = alphanum
	} else {
		pool = chars
	}

	for index := range bytes {
		bytes[index] = pool[random.Intn(len(pool))]
	}
	return string(bytes)
}

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}
