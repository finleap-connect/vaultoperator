# `vault-operator`

The vault operator provides provides several CRDs to ease the interaction with
secrets in vault. By only providing indirect access to kubernetes secrets this 
allows us to disallow secret access in production environments.

All CRDs should be available under the following API group:
```
vault.finleap.cloud/v1alpha
```

## CRD Kinds

### RBAC

To handle permissions for vault resources and specific vault instances we need
custom RBACs. `VaultRole` and `VaultRoleBinding` are always namespaced and can
only apply to permissions to a `Vault` instance in the same namespace!

By default all access is forbidden!

So for example:
A `VaultRole` and `VaultRoleBinding`, which can author all below resources, 
might be created and only allow all resources in the namespace of the related
`Vault` instance. However a second set of permissions might allow other 
namespaces to create indirect secrets via `VaultSecret` resources.

#### `VaultRole`

#### `VaultRoleBinding`

### Resources

#### `VaultSecret`

Example:
```yaml
apiVersion: vault.finleap.cloud/v1alpha1
kind: VaultSecret
metadata:
  name: myvaultsecret
spec:
  secretName: bydefaultsameasvaultsecret # optional
  data:
  - name: something
    generator: # optional, but if location does not exist will fail
      name: "string"|"bytes"|"password"|"rsa"|"ecdsa"|"uuid"
      args: [16] # arguments depend on the generator used, most only expect a single argument which is either length or size
    location: # required
      path: somevaultpath
      field: some field name
```

#### `VaultApprole`

#### `VaultPolicy`

#### `VaultTransitKey`

#### `Vault`

## Development

### Stage 1 `v1alpha1`

Only provide `VaultSecret` to create secrets indirectly by mapping them from
vault!
