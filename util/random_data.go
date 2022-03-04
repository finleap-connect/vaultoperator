// Copyright 2022 VaultOperator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"math/rand"
	"time"

	"github.com/sethvargo/go-password/password"
)

const allowedGlyphs = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.,/?!@#$%^&*()_+}|\":;'\\][=-"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = allowedGlyphs[rand.Intn(len(allowedGlyphs))]
	}
	return b
}

func RandString(n int) string {
	return string(RandBytes(n))
}

func RandPassword(n, digits, symbols int) (string, error) {
	return password.Generate(n, digits, symbols, false, true)
}
