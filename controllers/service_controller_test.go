package controllers

import (
	"context"
	"os"
	"testing"

	"github.com/kuadrant/authorino/api/v1beta1"
	"github.com/kuadrant/authorino/pkg/cache"
	mock_cache "github.com/kuadrant/authorino/pkg/cache/mocks"
	mocks "github.com/kuadrant/authorino/pkg/common/mocks"

	"github.com/golang/mock/gomock"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	service = v1beta1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "config.authorino.3scale.net/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service1",
			Namespace: "authorino",
		},
		Spec: v1beta1.ServiceSpec{
			Hosts: []string{"echo-api"},
			Identity: []*v1beta1.Identity{
				{
					Name: "keycloak",
					Oidc: &v1beta1.Identity_OidcConfig{
						Endpoint: "http://127.0.0.1:9001/auth/realms/demo",
					},
				},
			},
			Metadata: []*v1beta1.Metadata{
				{
					Name: "userinfo",
					UserInfo: &v1beta1.Metadata_UserInfo{
						IdentitySource: "keycloak",
					},
				},
				{
					Name: "resource-data",
					UMA: &v1beta1.Metadata_UMA{
						Endpoint: "http://127.0.0.1:9001/auth/realms/demo",
						Credentials: &v1.LocalObjectReference{
							Name: "secret",
						},
					},
				},
			},
			Authorization: []*v1beta1.Authorization{
				{
					Name: "main-policy",
					OPA: &v1beta1.Authorization_OPA{
						InlineRego: `allow {
            http_request.method == "GET"
            path = ["hello"]
          }

          allow {
            http_request.method == "GET"
            own_resource
          }

          allow {
            http_request.method == "GET"
            path = ["bye"]
            is_admin
          }

          own_resource {
            some greetingid
            path = ["greetings", greetingid]
            resource := object.get(metadata, "resource-data", [])[0]
            owner := object.get(object.get(resource, "owner", {}), "id", "")
            subject := object.get(identity, "sub", object.get(identity, "username", ""))
            owner == subject
          }

          is_admin {
            identity.realm_access.roles[_] == "admin"
          }`,
					},
				},
				{
					Name: "some-extra-rules",
					JSON: &v1beta1.Authorization_JSONPatternMatching{
						Rules: []v1beta1.Authorization_JSONPatternMatching_Rule{
							{
								Selector: "context.identity.role",
								Operator: "eq",
								Value:    "admin",
							},
							{
								Selector: "attributes.source.address.Address.SocketAddress.address",
								Operator: "eq",
								Value:    "80.133.21.75",
							},
						},
					}},
			},
		},
		Status: v1beta1.ServiceStatus{
			Ready: false,
		},
	}

	secret = v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "authorino",
		},
		Data: map[string][]byte{
			"clientID":     []byte("clientID"),
			"clientSecret": []byte("clientSecret"),
		},
	}
)

func TestMain(m *testing.M) {
	authServer := mocks.NewHttpServerMock("127.0.0.1:9001", map[string]mocks.HttpServerMockResponses{
		"/auth/realms/demo/.well-known/openid-configuration": {Status: 200, Body: `{ "issuer": "http://127.0.0.1:9001/auth/realms/demo" }`},
		"/auth/realms/demo/.well-known/uma2-configuration":   {Status: 200, Body: `{ "issuer": "http://127.0.0.1:9001/auth/realms/demo" }`},
	})
	defer authServer.Close()
	os.Exit(m.Run())
}

func setupEnvironment(t *testing.T, c cache.Cache) ServiceReconciler {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	// Create a fake client with a service and a secret.
	client := fake.NewFakeClientWithScheme(scheme, &service, &secret)

	return ServiceReconciler{
		Client:        client,
		Log:           ctrl.Log.WithName("reconcilerTest"),
		Scheme:        nil,
		Cache:         c,
		ServiceReader: client,
		ServiceWriter: client,
	}
}

func TestReconcilerOk(t *testing.T) {
	r := setupEnvironment(t, cache.NewCache())

	result, err := r.Reconcile(controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: service.Namespace,
			Name:      service.Name,
		},
	})

	if err != nil {
		t.Error(err)
	}

	// Result should be empty
	assert.DeepEqual(t, result, ctrl.Result{})
}

func TestReconcilerMissingSecret(t *testing.T) {
	r := setupEnvironment(t, cache.NewCache())

	_ = r.Client.Delete(context.TODO(), &secret)

	result, err := r.Reconcile(controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: service.Namespace,
			Name:      service.Name,
		},
	})

	// Error should be "secret" not found.
	assert.Check(t, errors.IsNotFound(err))
	// Result should be empty
	assert.DeepEqual(t, result, ctrl.Result{})
}

func TestReconcilerNotFound(t *testing.T) {
	r := setupEnvironment(t, cache.NewCache())

	// Let's try to reconcile a non existing object.
	result, err := r.Reconcile(controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: service.Namespace,
			Name:      "nonExistant",
		},
	})

	if err != nil {
		t.Error(err)
	}

	// Result should be empty
	assert.DeepEqual(t, result, ctrl.Result{})
}

func TestTranslateService(t *testing.T) {
	// TODO
}

func TestHostColllision(t *testing.T) {
	mockController := gomock.NewController(t)
	defer mockController.Finish()

	c := mock_cache.NewMockCache(mockController)
	r := setupEnvironment(t, c)

	c.EXPECT().FindId("echo-api").Return("other-namespace/other-service-with-same-host", true)

	result, err := r.Reconcile(controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: service.Namespace,
			Name:      service.Name,
		},
	})

	assert.DeepEqual(t, result, ctrl.Result{})
	assert.NilError(t, err)
}
