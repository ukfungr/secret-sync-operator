/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opsv1alpha1 "github.com/ukfungr/secret-sync-operator/api/v1alpha1"
	"github.com/ukfungr/secret-sync-operator/internal/provider"
)

type FakeSecretProvider struct {
	Data map[string][]byte
	Err error
}

func (f *FakeSecretProvider) GetSecret(
	ctx context.Context,
	key string,
) (map[string][]byte, error) {
	return f.Data, f.Err
}

type FakeProviderFactory struct {
	Provider provider.SecretProvider
}

func (f *FakeProviderFactory) GetProvider(
	providerType string,
) (provider.SecretProvider, error) {
	return f.Provider, nil
}

var _ = Describe("SecretSync Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		secretsync := &opsv1alpha1.SecretSync{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind SecretSync")
			err := k8sClient.Get(ctx, typeNamespacedName, secretsync)
			if err != nil && errors.IsNotFound(err) {
				resource := &opsv1alpha1.SecretSync{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: opsv1alpha1.SecretSyncSpec{
						Provider: opsv1alpha1.ProviderSpec{
							Type: "aws",
						},
						Remote: opsv1alpha1.RemoteSpec{
							Name: "test-secret",
						},
						Target: opsv1alpha1.TargetSpec{
							Name: "test-target-secret",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &opsv1alpha1.SecretSync{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SecretSync")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			targetSecret := &corev1.Secret{}

			err = k8sClient.Get(
				ctx,
				types.NamespacedName{
					Name: "test-target-secret",
					Namespace: resourceNamespace,
				},
				targetSecret,
			)

			if err == nil {
				By("Cleanup the target Secret")
				Expect(k8sClient.Delete(ctx, targetSecret)).To(Succeed())
			}
		})
		
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			fakeProvider := &FakeSecretProvider{
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("new-password"),
				},
			}

			fakeFactory := &FakeProviderFactory{
				Provider: fakeProvider,
			}

			controllerReconciler := &SecretSyncReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ProviderFactory: fakeFactory,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			var targetSecret corev1.Secret

			Expect(k8sClient.Get(
				ctx,
				types.NamespacedName{
					Name:      "test-target-secret",
					Namespace: typeNamespacedName.Namespace,
				},
				&targetSecret,
			)).To(Succeed())

			Expect(targetSecret.Data).To(Equal(map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("new-password"),
			}))

			
			// Test SecretSync status

			var updatedSecretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&updatedSecretSync,
			)).To(Succeed())
			
			Expect(updatedSecretSync.Status.Conditions).To(HaveLen(1))

			condition := updatedSecretSync.Status.Conditions[0]
			
			/** Current condition fields
			Type:               Ready
			Status:             True
			ObservedGeneration: 1
			LastTransitionTime: 2026-08-28... 
			Reason:              SecretSynced
			Message:             Secret successfully synchronized 
			*/
			Expect(condition.Type).To(Equal("Ready"))
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("SecretSynced"))
			Expect(condition.Message).To(Equal("Secret successfully synchronized"))

		})

		It("should not update the target secret when data is already synchronized", func() {
			By("creating the target secret")

			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target-secret",
					Namespace: resourceNamespace,
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("new-password"),
				},
			}

			Expect(k8sClient.Create(ctx, targetSecret)).To(Succeed())

			By("creating the fake provider")

			fakeProvider := &FakeSecretProvider{
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("new-password"),
				},
			}

			fakeFactory := &FakeProviderFactory{
				Provider: fakeProvider,
			}

			controllerReconciler := &SecretSyncReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ProviderFactory: fakeFactory,
			}

			By("reconciling the resource")

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			By("verifying the target secret")

			var result corev1.Secret

			Expect(k8sClient.Get(
				ctx,
				types.NamespacedName{
					Name:      "test-target-secret",
					Namespace: resourceNamespace,
				},
				&result,
			)).To(Succeed())

			Expect(result.Data).To(Equal(map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("new-password"),
			}))

			By("verifying the SecretSync status")

			var updatedSecretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&updatedSecretSync,
			)).To(Succeed())

			Expect(updatedSecretSync.Status.Conditions).To(HaveLen(1))

			condition := updatedSecretSync.Status.Conditions[0]

			Expect(condition.Type).To(Equal("Ready"))
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("SecretSynced"))
			Expect(condition.Message).To(Equal("Secret is already synchronized"))
		})

		It("should update the target secret when data is different", func() {
			By("creating the target secret with old data")

			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target-secret",
					Namespace: resourceNamespace,
				},
				Data: map[string][]byte{
					"username": []byte("old-user"),
					"password": []byte("old-password"),
				},
			}

			Expect(k8sClient.Create(ctx, targetSecret)).To(Succeed())

			By("creating the fake provider with new data")

			fakeProvider := &FakeSecretProvider{
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("new-password"),
				},
			}

			fakeFactory := &FakeProviderFactory{
				Provider: fakeProvider,
			}

			controllerReconciler := &SecretSyncReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ProviderFactory: fakeFactory,
			}

			By("reconciling the resource")

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			By("verifying the target secret was updated")

			var result corev1.Secret

			Expect(k8sClient.Get(
				ctx,
				types.NamespacedName{
					Name:      "test-target-secret",
					Namespace: resourceNamespace,
				},
				&result,
			)).To(Succeed())

			Expect(result.Data).To(Equal(map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("new-password"),
			}))

			By("verifying the SecretSync status")

			var updatedSecretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&updatedSecretSync,
			)).To(Succeed())

			Expect(updatedSecretSync.Status.Conditions).To(HaveLen(1))

			condition := updatedSecretSync.Status.Conditions[0]

			Expect(condition.Type).To(Equal("Ready"))
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("SecretSynced"))
			Expect(condition.Message).To(Equal("Secret successfully synchronized"))
		})

		It("should return an error when the provider is unsupported", func() {
			By("creating a SecretSync with an unsupported provider")

			var secretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&secretSync,
			)).To(Succeed())

			secretSync.Spec.Provider.Type = "unsupported"

			Expect(k8sClient.Update(ctx, &secretSync)).To(Succeed())

			By("reconciling the resource")

			controllerReconciler := &SecretSyncReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ProviderFactory: &provider.ProviderFactory{}, // using real Factory provider to test the logic there that is bypassed by the FakeFactoryProvider other tests are using here
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported provider type"))

			var updatedSecretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&updatedSecretSync,
			)).To(Succeed())

			Expect(updatedSecretSync.Status.Conditions).To(HaveLen(1))

			condition := updatedSecretSync.Status.Conditions[0]

			Expect(condition.Type).To(Equal("Ready"))
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal("SyncFailed"))
			Expect(condition.Message).To(ContainSubstring("unsupported provider type"))
		})

		It("should return an error when the provider fails to get the secret", func() {
			By("creating a fake provider that returns an error")

			fakeProvider := &FakeSecretProvider{
				Err: fmt.Errorf("failed to retrieve remote secret"),
			}

			fakeFactory := &FakeProviderFactory{
				Provider: fakeProvider,
			}

			controllerReconciler := &SecretSyncReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				ProviderFactory: fakeFactory,
			}

			By("reconciling the resource")

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve remote secret"))

			var updatedSecretSync opsv1alpha1.SecretSync

			Expect(k8sClient.Get(
				ctx,
				typeNamespacedName,
				&updatedSecretSync,
			)).To(Succeed())
			
			Expect(updatedSecretSync.Status.Conditions).To(HaveLen(1))

			condition := updatedSecretSync.Status.Conditions[0]

			Expect(condition.Type).To(Equal("Ready"))
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal("SyncFailed"))
			Expect(condition.Message).To(Equal("failed to retrieve remote secret"))
		})
	})
})
