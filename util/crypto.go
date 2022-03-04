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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

func GenerateRSA(bits int32) ([]byte, error) {
	pk, err := rsa.GenerateKey(rand.Reader, int(bits))
	if err != nil {
		return nil, err
	}
	x509Enc := x509.MarshalPKCS1PrivateKey(pk)
	pemEnc := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509Enc})
	return pemEnc, nil
}

func GenerateECDSA(c int32) ([]byte, error) {
	var curve elliptic.Curve
	switch c {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, errors.New("unknown curve")
	}
	pk, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	x509Enc, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return nil, err
	}
	pemEnc := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: x509Enc})
	return pemEnc, nil
}
