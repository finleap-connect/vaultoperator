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

package vault

// Initial version from: authn-authz/customer-credential-service/blob/develop/pkg/vault/vault.go

import (
	"github.com/hashicorp/vault/api"
)

// AuthMethod specifies an authentication method for the Hashicorp Vault API.
type AuthMethod interface {
	// Login creates a new authentication token.
	Login(*Client) (*api.Secret, error)
	// Name returns the name of the authentication method.
	Name() string
	// Check if token is renewable
	IsRenewable() bool
}

// AppRoleAuth implements the the AppRole authentication method.
// See: https://www.vaultproject.io/docs/auth/approle.html
type AppRoleAuth struct {
	RoleID   string
	SecretID string
}

func (a *AppRoleAuth) Login(c *Client) (*api.Secret, error) {
	return c.Logical().Write("/auth/approle/login", map[string]interface{}{
		"role_id":   a.RoleID,
		"secret_id": a.SecretID,
	})
}

func (a *AppRoleAuth) Name() string { return "AppRole" }

func (a *AppRoleAuth) IsRenewable() bool { return true }

type TokenAuth struct {
	Token string
}

func (a *TokenAuth) Login(c *Client) (*api.Secret, error) {
	return &api.Secret{Auth: &api.SecretAuth{ClientToken: a.Token}}, nil
}

func (a *TokenAuth) Name() string { return "Token" }

func (a *TokenAuth) IsRenewable() bool { return false }
