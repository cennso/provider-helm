/*
Copyright 2025 The Crossplane Authors.

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

package helm

import (
	"context"
	"testing"

	kconfig "github.com/crossplane-contrib/provider-kubernetes/pkg/kube/config"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	resourcefake "github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	namespacedapis "github.com/crossplane-contrib/provider-helm/apis/namespaced"
	namespacedv1beta1 "github.com/crossplane-contrib/provider-helm/apis/namespaced/v1beta1"
)

func Test_resolveProviderConfigModern(t *testing.T) {
	sch := runtime.NewScheme()
	if err := namespacedapis.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}

	newMg := func(name, ns string, ref *xpv1.ProviderConfigReference) resource.ModernManaged {
		return &resourcefake.ModernManaged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			TypedProviderConfigReferencer: resourcefake.TypedProviderConfigReferencer{
				Ref: ref,
			},
		}
	}

	type want struct {
		spec *kconfig.ProviderConfigSpec
		err  error
	}
	type args struct {
		crClient kclient.Client
		mg       resource.ModernManaged
		mt       ModernTracker
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"NilProviderConfigRef": {
			args: args{
				crClient: nil,
				mg:       newMg("mg", "ns", nil),
			},
			want: want{
				err: errors.New(errProviderConfigNotSet),
			},
		},
		"FailsOnGetProviderConfigError": {
			args: args{
				crClient: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(sch),
					MockGet: func(ctx context.Context, key kclient.ObjectKey, obj kclient.Object) error {
						return errors.New("boom")
					},
				},
				mg: newMg("mg", "ns", &xpv1.ProviderConfigReference{
					Name: "pc",
					Kind: namespacedv1beta1.ProviderConfigKind,
				}),
			},
			want: want{
				err: errors.Wrap(errors.New("boom"), errGetProviderConfig),
			},
		},
		"FailsOnTrackingModernManaged": {
			args: args{
				crClient: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(sch),
					MockGet: func(ctx context.Context, key kclient.ObjectKey, obj kclient.Object) error {
						return nil
					},
				},
				mg: newMg("mg", "ns", &xpv1.ProviderConfigReference{
					Name: "pc",
					Kind: namespacedv1beta1.ProviderConfigKind,
				}),
				mt: ModernTrackerFn(func(ctx context.Context, mg resource.ModernManaged) error {
					return errors.New("tracking boom")
				}),
			},
			want: want{
				err: errors.Wrap(errors.New("tracking boom"), errFailedToTrackUsage),
			},
		},
		"ReturnsExpectedProviderConfigSpec": {
			args: args{
				crClient: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(sch),
					MockGet: func(ctx context.Context, key kclient.ObjectKey, obj kclient.Object) error {
						obj.(*namespacedv1beta1.ProviderConfig).Spec.Credentials.Source = xpv1.CredentialsSourceSecret
						return nil
					},
				},
				mg: newMg("mg", "ns", &xpv1.ProviderConfigReference{
					Name: "pc",
					Kind: namespacedv1beta1.ProviderConfigKind,
				}),
				mt: ModernTrackerFn(func(ctx context.Context, mg resource.ModernManaged) error {
					return nil
				}),
			},
			want: want{
				err: nil,
				spec: &kconfig.ProviderConfigSpec{
					Credentials: kconfig.ProviderCredentials{
						Source: xpv1.CredentialsSourceSecret,
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, gotErr := resolveProviderConfigModern(t.Context(), tt.args.crClient, tt.args.mg, tt.args.mt)
			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("resolveProviderConfigModern() error, -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.spec, got); diff != "" {
				t.Errorf("resolveProviderConfigModern() -want, +got:\n%s", diff)
			}
		})
	}
}
