package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	vaultv1alpha1 "github.com/finleap-connect/vaultoperator/api/v1alpha1"
	"github.com/finleap-connect/vaultoperator/vault"
)

type UpdateSpecFunc = func(spec *vaultv1alpha1.VaultSecretSpec)

func WithVaultPath(path string) UpdateSpecFunc {
	return func(spec *vaultv1alpha1.VaultSecretSpec) {
		for _, data := range spec.Data {
			if data.Location == nil {
				continue
			}
			data.Location.Path = path
		}
	}
}

func newVaultSecret(updates ...UpdateSpecFunc) *vaultv1alpha1.VaultSecret {
	vs := &vaultv1alpha1.VaultSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      newTestName(),
		},
		Spec: vaultv1alpha1.VaultSecretSpec{
			Data: []vaultv1alpha1.VaultSecretData{
				{
					Name: "foo",
					Location: &vaultv1alpha1.VaultSecretLocation{
						Path:  "app/test/bar",
						Field: "baz",
					},
				},
			},
		},
	}
	for _, f := range updates {
		f(&vs.Spec)
	}
	return vs
}

func newVaultSecretFromPath() *vaultv1alpha1.VaultSecret {
	return &vaultv1alpha1.VaultSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      newTestName(),
		},
		Spec: vaultv1alpha1.VaultSecretSpec{
			DataFrom: []vaultv1alpha1.VaultSecretDataRef{
				{
					Path: "app/test/bar",
				},
				{
					Path:              "app/test/foo",
					CollisionStrategy: vaultv1alpha1.OverwriteCollision,
				},
			},
		},
	}
}

func newBinaryVaultSecret() *vaultv1alpha1.VaultSecret {
	return &vaultv1alpha1.VaultSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      newTestName(),
		},
		Spec: vaultv1alpha1.VaultSecretSpec{
			Data: []vaultv1alpha1.VaultSecretData{
				{
					Name: "foobin",
					Location: &vaultv1alpha1.VaultSecretLocation{
						Path:     "app/test/binbar",
						Field:    "baz",
						IsBinary: true,
					},
				},
			},
		},
	}
}

func newSecret(name string) *corev1.Secret {
	s := &corev1.Secret{}
	s.ObjectMeta.Name = name
	s.ObjectMeta.Namespace = testNamespace
	s.Data = map[string][]byte{
		"foo": []byte("nothing"),
		"bar": []byte("nothingelse"),
	}
	return s
}

func newRequestFor(vs *vaultv1alpha1.VaultSecret) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: vs.Namespace,
			Name:      vs.Name,
		},
	}
}

func namespacedName(obj runtime.Object) types.NamespacedName {
	accessor, err := meta.Accessor(obj)
	Expect(err).ToNot(HaveOccurred())
	return types.NamespacedName{
		Namespace: accessor.GetNamespace(),
		Name:      accessor.GetName(),
	}
}

func mustCreateNewVaultSecret(updates ...UpdateSpecFunc) *vaultv1alpha1.VaultSecret {
	vs := newVaultSecret(updates...)
	Expect(k8sClient.Create(context.Background(), vs)).To(Succeed())
	return vs
}

func mustReconcile(vs *vaultv1alpha1.VaultSecret) ctrl.Result {
	req := newRequestFor(vs)
	result, err := testVSR.Reconcile(context.Background(), req)
	Expect(err).ToNot(HaveOccurred())
	return result
}

func mustNotReconcile(vs *vaultv1alpha1.VaultSecret, expected interface{}) ctrl.Result {
	req := newRequestFor(vs)
	result, err := testVSR.Reconcile(context.Background(), req)
	if expected == nil {
		Expect(err).To(HaveOccurred())
	} else {
		Expect(err).To(MatchError(expected))
	}
	return result
}

var _ = Describe("VaultSecretReconciler", func() {
	ctx := context.Background()
	It("can create VaultSecrets", func() {
		Context("with missing data", func() {
			Expect(k8sClient.Create(ctx, &vaultv1alpha1.VaultSecret{})).ToNot(Succeed())
		})
		Context("with valid data", func() {
			mustCreateNewVaultSecret()
		})
		Context("with valid dataFrom", func() {
			vs := newVaultSecretFromPath()
			Expect(k8sClient.Create(ctx, vs)).To(Succeed())
		})
		Context("with valid binary data", func() {
			Expect(k8sClient.Create(ctx, newBinaryVaultSecret())).To(Succeed())
		})
	})
	It("can process VaultSecrets", func() {
		Context("which are just created", func() {
			res := mustReconcile(mustCreateNewVaultSecret())
			Expect(res.Requeue).To(BeFalse())
		})
	})
	It("can process VaultSecrets with dataFrom", func() {
		Context("which are just created", func() {
			vs := newVaultSecretFromPath()
			Expect(k8sClient.Create(ctx, vs)).To(Succeed())
			res, err := testVSR.Reconcile(context.Background(), newRequestFor(vs))
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
		})
	})
	It("can handle finalizer", func() {
		Context("created for new secret", func() {
			vs := mustCreateNewVaultSecret()

			before := &vaultv1alpha1.VaultSecret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), before)).To(Succeed())
			Expect(before.ObjectMeta.Finalizers).NotTo(ContainElement(finalizerName))

			mustReconcile(vs)

			after := &vaultv1alpha1.VaultSecret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), after)).To(Succeed())
			Expect(after.ObjectMeta.Finalizers).To(ContainElement(finalizerName))
		})
		Context("deleted on cleanup", func() {
			vs := mustCreateNewVaultSecret()
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())

			before := &vaultv1alpha1.VaultSecret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), before)).To(Succeed())
			Expect(before.ObjectMeta.Finalizers).To(ContainElement(finalizerName))

			Expect(k8sClient.Delete(ctx, before)).To(Succeed())

			mustReconcile(vs)

			after := &vaultv1alpha1.VaultSecret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), after)).ToNot(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).ToNot(Succeed())
		})
		Context("handle if secret is gone already", func() {
			vs := mustCreateNewVaultSecret()
			mustReconcile(vs)

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), secret)).To(Succeed())

			vaultSecret := &vaultv1alpha1.VaultSecret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), vaultSecret)).To(Succeed())
			Expect(vaultSecret.ObjectMeta.Finalizers).To(ContainElement(finalizerName))

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName(vs), secret)).ToNot(Succeed())

			Expect(k8sClient.Delete(ctx, vaultSecret)).To(Succeed())
			mustReconcile(vs)
			Expect(k8sClient.Get(ctx, namespacedName(vs), vaultSecret)).ToNot(Succeed())
		})
	})
	It("can handle related secrets", func() {
		Context("create for new secret", func() {
			vs := mustCreateNewVaultSecret()
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["foo"]).To(Equal([]byte("fizzbuzz")))
		})
		Context("create for new secret dataFrom", func() {
			vs := newVaultSecretFromPath()
			err := k8sClient.Create(ctx, vs)
			Expect(err).ToNot(HaveOccurred())
			req := newRequestFor(vs)
			_, err = testVSR.Reconcile(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())
			s := &corev1.Secret{}
			err = k8sClient.Get(ctx, req.NamespacedName, s)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Data["bax"]).To(Equal([]byte("fixxbaxx")))
			Expect(s.Data["baz"]).To(Equal([]byte("foo")))
		})
		Context("create for new binary secret", func() {
			vs := newBinaryVaultSecret()
			s := newSecret(vs.ObjectMeta.Name)

			Expect(k8sClient.Create(ctx, vs)).To(Succeed())

			mustReconcile(vs)

			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["foobin"]).To(Equal([]byte("fizzbuzzb")))
		})
		Context("create for existing secret", func() {
			vs := newVaultSecret()
			s := newSecret(vs.ObjectMeta.Name)
			Expect(k8sClient.Create(ctx, s)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["foo"]).To(Equal([]byte("nothing")))

			Expect(k8sClient.Create(ctx, vs)).To(Succeed())
			mustNotReconcile(vs, fmt.Sprintf("secrets \"%s\" already exists", vs.ObjectMeta.Name))
		})
		Context("create for existing secret dataFrom", func() {
			vs := newVaultSecretFromPath()
			s := newSecret(vs.ObjectMeta.Name)
			req := newRequestFor(vs)
			Expect(k8sClient.Create(ctx, s)).To(Succeed())
			Expect(k8sClient.Get(ctx, req.NamespacedName, s)).To(Succeed())
			Expect(s.Data["foo"]).To(Equal([]byte("nothing")))
			Expect(s.Data["bar"]).To(Equal([]byte("nothingelse")))
		})
		Context("non-existant vault path and no generate", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Path = "app/test/notexisting"
			})

			_, err := testVSR.Reconcile(context.Background(), newRequestFor(vs))
			Expect(err).To(MatchError(vault.ErrNotFound))
		})
		Context("existant vault path with generator update unnecessary", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Generator = &vaultv1alpha1.VaultSecretGenerator{
					Name: "string",
					Args: []int32{32},
				}
			})
			mustReconcile(vs)

			Expect(testVaultClient.Get("app/test/bar", "baz", 0)).To(Equal("fizzbuzz"))
		})
		Context("existant vault path with generator update necessary", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Field = "other"
				spec.Data[0].Generator = &vaultv1alpha1.VaultSecretGenerator{
					Name: "string",
					Args: []int32{32},
				}
			})
			mustReconcile(vs)

			Expect(testVaultClient.Get("app/test/bar", "baz", 1)).To(Equal("buzzfizz"))
			Expect(testVaultClient.Get("app/test/bar", "other", 0)).To(HaveLen(32))
		})
		Context("non-existant vault path with generator", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Path = "app/test/generate"
				spec.Data[0].Generator = &vaultv1alpha1.VaultSecretGenerator{
					Name: "string",
					Args: []int32{32},
				}
			})
			mustReconcile(vs)

			Expect(testVaultClient.Get("app/test/generate", "baz", 0)).To(HaveLen(32))
		})
	})
	It("can handle dockerconfigjson", func() {
		Context("new secret", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Name = ".dockerconfigjson"
				spec.Data[0].Location.Path = "app/test/docker"
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
			Expect(s.Data[".dockerconfigjson"]).To(Equal([]byte(testDockerConfigJSON)))
		})
	})
	It("can set secret type", func() {
		Context("new secret", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.SecretType = "kubernetes.io/tls"
				spec.Data[0].Name = corev1.TLSCertKey
				spec.Data = append(spec.Data, vaultv1alpha1.VaultSecretData{
					Name: corev1.TLSPrivateKeyKey,
					Location: &vaultv1alpha1.VaultSecretLocation{
						Path:  "app/test/bar",
						Field: "baz",
					},
				})
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Type).To(Equal(corev1.SecretTypeTLS))
		})
	})
	It("can use templating", func() {
		Context("with variables", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data = append(spec.Data, vaultv1alpha1.VaultSecretData{
					Name: "template",
					Variables: []vaultv1alpha1.VaultSecretVariable{
						{
							Name: "test",
							Location: &vaultv1alpha1.VaultSecretLocation{
								Path:  "app/test/bar",
								Field: "baz",
							},
						},
					},
					Template: "test{{.test}}",
				})
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["template"]).To(Equal([]byte("testfizzbuzz")))
		})
		Context("without variables", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data = append(spec.Data, vaultv1alpha1.VaultSecretData{
					Name:     "template",
					Template: "static",
				})
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["template"]).To(Equal([]byte("static")))
		})
	})
	It("uses correct version", func() {
		Context("specific version", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Version = 1
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["foo"]).To(Equal([]byte("buzzfizz")))
		})
		Context("latest version", func() {
			vs := mustCreateNewVaultSecret()
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())
			Expect(s.Data["foo"]).To(Equal([]byte("fizzbuzz")))
		})
	})
	It("can generate secrets", func() {
		Context("when type is 'uuid'", func() {
			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Field = "uuid"
				spec.Data[0].Generator = &vaultv1alpha1.VaultSecretGenerator{
					Name: "uuid",
					Args: []int32{},
				}
			})
			mustReconcile(vs)

			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName(vs), s)).To(Succeed())

			_, err := uuid.Parse(string(s.Data["foo"]))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	It("rejects vault paths", func() {
		for _, test := range []struct {
			desc string
			path string
			err  error
		}{
			{
				desc: "with prefix other than app or cert",
				path: "foo/bar/baz",
				err:  ErrPermissionDenied,
			},
			{
				desc: "with app-prefix shorter than 3 segments",
				path: "app/dev",
				err:  ErrInvalidVaultPath,
			},
			{
				desc: "with unsupported scope",
				path: "app/foo/bar",
				err:  ErrPermissionDenied,
			},
		} {
			Context(test.desc, func() {
				mustNotReconcile(mustCreateNewVaultSecret(WithVaultPath(test.path)), test.err)
			})
		}
	})
	It("allows vault paths", func() {

		for _, test := range []struct {
			desc       string
			path       string
			updateFunc UpdateSpecFunc
		}{
			{
				desc: "with cert prefix to have less than 3 segments",
				path: "cert/root-ca",
			},
			{
				desc: "with scope matching current namespace",
				path: "app/" + testNamespace + "/foo",
			},
			{
				desc: "with scope matching pre-defined identifier",
				path: "app/finleap/foo",
			},
		} {
			Context(test.desc, func() {
				mustReconcile(mustCreateNewVaultSecret(WithVaultPath(test.path), func(spec *vaultv1alpha1.VaultSecretSpec) {
					spec.Data[0].Generator = &vaultv1alpha1.VaultSecretGenerator{
						Name: "uuid",
						Args: []int32{},
					}
				}))
			})
		}
	})
	It("can not access vault", func() {
		Context("outside of the specified vault namespace", func() {
			if !testWithEnterprise {
				Skip("no Vault enterprise binary present => Skip namespace tests")
			}

			vs := mustCreateNewVaultSecret(func(spec *vaultv1alpha1.VaultSecretSpec) {
				spec.Data[0].Location.Path = "app/test/only"
				spec.Data[0].Location.Field = "for"
			})
			mustNotReconcile(vs, nil)
		})
	})
})
