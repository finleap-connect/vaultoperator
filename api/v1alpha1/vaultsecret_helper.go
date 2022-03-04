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

package v1alpha1

// +kubebuilder:object:generate=false
type AnyVaultSecretData interface {
	GetName() string
	GetLocation() *VaultSecretLocation
	GetGenerator() *VaultSecretGenerator
	GetCollisionStrategy() FieldCollisionStrategy
}

func (d *VaultSecretData) GetName() string {
	return d.Name
}

func (d *VaultSecretData) GetLocation() *VaultSecretLocation {
	return d.Location
}

func (d *VaultSecretData) GetGenerator() *VaultSecretGenerator {
	return d.Generator
}

func (d *VaultSecretData) GetCollisionStrategy() FieldCollisionStrategy {
	return ErrorOnCollision
}

func (d *VaultSecretVariable) GetName() string {
	return d.Name
}

func (d *VaultSecretVariable) GetLocation() *VaultSecretLocation {
	return d.Location
}

func (d *VaultSecretVariable) GetGenerator() *VaultSecretGenerator {
	return d.Generator
}

func (d *VaultSecretVariable) GetCollisionStrategy() FieldCollisionStrategy {
	return ErrorOnCollision
}

func (d *VaultSecretDataRef) GetName() string {
	return d.Path
}

func (d *VaultSecretDataRef) GetLocation() *VaultSecretLocation {
	return &VaultSecretLocation{Path: d.Path, Version: d.Version}
}

func (d *VaultSecretDataRef) GetGenerator() *VaultSecretGenerator {
	return nil
}

func (d *VaultSecretDataRef) GetCollisionStrategy() FieldCollisionStrategy {
	if d.CollisionStrategy != "" {
		return d.CollisionStrategy
	}
	return ErrorOnCollision
}
