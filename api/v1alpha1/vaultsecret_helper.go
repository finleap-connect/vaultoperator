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
