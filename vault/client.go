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

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

const initialTokenTimeout = 10 * time.Second

type Client struct {
	*api.Client
	log          logr.Logger
	tokenHandler *TokenHandler
}

func NewClient(addr string, namespace string, method AuthMethod) (*Client, error) {
	var err error
	c := &Client{log: ctrl.Log.WithName("VaultClient")}
	cfg := api.DefaultConfig()
	cfg.Address = addr
	c.Client, err = api.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not create vault client")
	}
	if namespace != "" {
		c.SetNamespace(namespace)
	}
	if method != nil {
		c.log.Info("Configure auth method.", "name", method.Name())
		c.tokenHandler = NewTokenHandler(c, method)
		if err := c.tokenHandler.WaitForToken(initialTokenTimeout); err != nil {
			return nil, err
		}
	} else {
		return nil, ErrAuthMethodNotProvided
	}
	if c.Token() == "" {
		return nil, ErrMissingToken
	}
	return c, nil
}

func (c *Client) GetAll(path string, version int) (map[string]string, error) {
	path = toDataPath(path)
	params := map[string][]string{}
	if version > 0 {
		params["version"] = []string{strconv.Itoa(version)}
	}
	secret, err := c.Client.Logical().ReadWithData(path, params)
	if err != nil {
		return nil, err
	}
	fields, err := getFieldsFromSecret(secret)
	if err != nil {
		return nil, err
	}
	return fields, nil
}

func (c *Client) Get(path, field string, version int) (string, error) {
	path = toDataPath(path)
	params := map[string][]string{}
	if version > 0 {
		params["version"] = []string{strconv.Itoa(version)}
	}
	secret, err := c.Client.Logical().ReadWithData(path, params)
	if err != nil {
		return "", err
	}
	value, err := getFieldFromSecret(secret, field)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (c *Client) CreateOrUpdate(path string, data map[string]interface{}) error {
	path = toDataPath(path)
	secret, err := c.Client.Logical().Read(path)
	if err != nil {
		return err
	}

	cas := 1
	merged := map[string]interface{}{}
	if secret != nil && secret.Data != nil {
		if raw, ok := secret.Data["data"]; ok {
			if data, ok := raw.(map[string]interface{}); ok {
				for k, v := range data {
					merged[k] = v
				}
			}
		}
		if raw, ok := secret.Data["metadata"]; ok {
			if data, ok := raw.(map[string]interface{}); ok {
				if v1, ok := data["version"]; ok {
					if v2, ok := v1.(json.Number); ok {
						version, err := v2.Int64()
						if err == nil {
							cas = int(version)
						}
					}
				}
			}
		}
	}
	for k, v := range data {
		merged[k] = v
	}
	req := c.NewRequest("PUT", "/v1/"+path)
	payload := map[string]interface{}{
		"data": merged,
		"cas":  cas,
	}
	if err := req.SetJSONBody(payload); err != nil {
		return err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	res, err := c.RawRequestWithContext(ctx, req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() { c.tokenHandler.Close() }

func toDataPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path
	} else {
		return strings.Join(append([]string{parts[0], "data"}, parts[1:]...), "/")
	}
}

func getFieldFromSecret(secret *api.Secret, field string) (string, error) {
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 {
		return "", ErrNotFound
	}
	if d1, ok := secret.Data["data"]; ok {
		if d2, ok := d1.(map[string]interface{}); ok {
			if v1, ok := d2[field]; ok {
				if v2, ok := v1.(string); ok {
					return v2, nil
				}
			}
		}
	}
	return "", ErrNotFound
}

func getFieldsFromSecret(secret *api.Secret) (map[string]string, error) {
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 {
		return nil, ErrNotFound
	}

	fields := make(map[string]string)
	if d1, ok := secret.Data["data"]; ok {
		if d2, ok := d1.(map[string]interface{}); ok {
			for field, value := range d2 {
				if v2, ok := value.(string); ok {
					fields[field] = v2
				}
			}
		}
	}
	return fields, nil
}

func GetIsBinaryKey(key string) string {
	return fmt.Sprintf(".%s_isBinary", key)
}
