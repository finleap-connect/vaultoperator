# `vault-operator`

[![Build status](https://github.com/finleap-connect/vaultoperator/actions/workflows/golang.yaml/badge.svg)](https://github.com/finleap-connect/vaultoperator/actions/workflows/golang.yaml)
[![Coverage Status](https://coveralls.io/repos/github/finleap-connect/vaultoperator/badge.svg?branch=main)](https://coveralls.io/github/finleap-connect/vaultoperator?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/finleap-connect/vaultoperator)](https://goreportcard.com/report/github.com/finleap-connect/vaultoperator)
[![Go Reference](https://pkg.go.dev/badge/github.com/finleap-connect/vaultoperator.svg)](https://pkg.go.dev/github.com/finleap-connect/vaultoperator)
[![GitHub release](https://img.shields.io/github/release/finleap-connect/vaultoperator.svg)](https://github.com/finleap-connect/vaultoperator/releases)

The `vault-operator` provides several CRDs to interact securely and indirectly with secrets.
## Details

Currently only _stage 1_ is implemented, which includes the `VaultSecret`-CRD.

For future feature and planning refer to [DESIGN.md](./DESIGN.md).

### `VaultSecret`

To give indirect control over secrets the `VaultSecret` can be used. For each
field name in a `Secret` it refers to a location in _vault_ and will pull the data and write it to the secret.

If the data in _vault_ does _not_ exist, it will be created if a `generator` is
provided. Currently several generators are implemented:

* `string` generates a random string with length `args[0]`
* `bytes` generates random bytes with length `args[0]`
* `password` special form of string generations where `args[0]` is the length and is mandatory. `args[1]` optionally specifies the number of digits and `args[2]` optionally defines the number of symbols.
* `rsa` generates RSA private key with bit size `args[0]` (encoded as PEM)
* `ecdsa` generates EC private key with curve `args[0]` (encoded as PEM)

Locations in the vault are given by the `path` and the `field` within the entry.
Optionally the version of the entry may be given. This is only valid if the secret
engine of the entry is of the type `KV v2`. To ensure reproducable deployments, 
the version number should be set when ever possible.

Furthermore simplified permission control exists. Every `VaultSecret` can access
shared spaces which can be configured via the Helm Chart, but otherwise only namespaced sub-paths
are permitted, e.g. `VaultSecret` in `mynamespace` can access `app/mynamespace`.

Example:

```yaml
apiVersion: vault.finleap.cloud/v1alpha1
kind: VaultSecret
metadata:
  name: myvaultsecret
  namespace: mynamespace
spec:
  secretName: name-of-generated-secret  # optional, default it is the same as the name of the VaultSecret
  data: # optional if dataFrom is specified
  - name: something
    generator: # optional
      name: "string"
      args: [16]
    location: # required, if variables and template not provided
      path: app/test/foo
      field: bar
  - name: morecomplex
    variables: # required, if location not provided
    - name: "test"
      location:
        path: app/test/fizz
        field: buzz
        isBinary: 1 # optional
        version: 1 # optional
      generator: # optional same as above
    template: |- # required if location not provided
      asdasd {{.test}}
  dataFrom: # optional if data is specified, gets all fields under a given vault path
  - path: app/test/bar
    version: 1 #optional
    collisionStrategy: "Error" #optional
    # Valid values are:
    # - "Error" (default): Errors if a field on this vault secret already exists on the resulting K8s secret
    # - "Ignore": Value from this vault secret will be ignored if the same field already exists on resulting K8s secret
    # - "Overwrite": Value from this vault secret will override an already existing field on the resulting K8s secret
  - path: app/test/bazz
    version: 1 #optional
    collisionStrategy: "Overwrite" #optional
```

#### Special cases

1. If the VaultSecret only contains a single data element with the name `.dockerconfigjson`,
the created secret will have the type `kubernetes.io/dockerconfigjson` instead of `Opaque`.
2. When using a generator it is not allowed to set a fixed version. Renewal for generated secrets is an ongoing discussion. The generator will only run if the concrete field in the secret does not yet exist in vault.
3. If `dataFrom` is used, multiple paths in vault can be specified and all fields of the paths in vault will be joined in one secret. As collisions can occure, it is possible to define the strategy how to handle these. The default strategy is `Error`.

## Development

This project utilizes [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
and therefore please refer to its [documentation](https://github.com/kubernetes-sigs/kubebuilder/blob/master/designs/simplified-scaffolding.md) to understand the scaffolding (there
are significant differences to the [standard layout](https://github.com/golang-standards/project-layout)).

### Prerequisites

The test suite needs the kubebuilder assets. If they are not installed in the default
path make sure to set `KUBEBUILDER_ASSETS` before running tests.
Similarly the vault CLI needs to be setup, if it is outside your `PATH` make sure to
set `VAULT_ASSETS` to the directory containing the vault executable.

