// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package service

import (
	"context"
	"github.com/Azure/aks-app-routing-operator/pkg/controller/metrics"
	"github.com/Azure/aks-app-routing-operator/pkg/controller/testutils"
	"testing"

	"github.com/Azure/aks-app-routing-operator/pkg/manifests"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Azure/aks-app-routing-operator/pkg/util"
)

const (
	icName = "test.ic.name"
)

func TestIngressReconcilerIntegration(t *testing.T) {
	svc := &corev1.Service{}
	svc.UID = "test-svc-uid"
	svc.Name = "test-service"
	svc.Namespace = "test-ns"
	svc.Spec.Ports = []corev1.ServicePort{{
		Port:       123,
		TargetPort: intstr.FromInt(234),
	}}

	c := fake.NewClientBuilder().WithObjects(svc).Build()
	p := &NginxIngressReconciler{client: c, ingConfig: &manifests.NginxIngressConfig{IcName: icName}}

	ctx := context.Background()
	ctx = logr.NewContext(ctx, logr.Discard())

	// No ingress is created for service without any of our annotations
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}}
	beforeErrCount := testutils.GetErrMetricCount(t, ingressControllerName)
	beforeReconcileCount := testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess)
	_, err := p.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, testutils.GetErrMetricCount(t, ingressControllerName), beforeErrCount)
	assert.Greater(t, testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess), beforeReconcileCount)

	ing := &netv1.Ingress{}
	ing.Name = svc.Name
	ing.Namespace = svc.Namespace
	assert.True(t, errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(ing), ing)))

	// Add required annotations and prove the expected ingress is created
	svc.Annotations = map[string]string{
		"kubernetes.azure.com/ingress-host":          "test-host",
		"kubernetes.azure.com/tls-cert-keyvault-uri": "test-cert-uri",
	}
	require.NoError(t, c.Update(ctx, svc))

	beforeErrCount = testutils.GetErrMetricCount(t, ingressControllerName)
	beforeReconcileCount = testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess)
	_, err = p.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, testutils.GetErrMetricCount(t, ingressControllerName), beforeErrCount)
	assert.Greater(t, testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess), beforeReconcileCount)

	pt := netv1.PathTypePrefix
	expected := &netv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            svc.Name,
			Namespace:       svc.Namespace,
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Controller: util.BoolPtr(true),
				Kind:       "Service",
				Name:       svc.Name,
				UID:        svc.UID,
			}},
			Annotations: map[string]string{
				"kubernetes.azure.com/tls-cert-keyvault-uri":        "test-cert-uri",
				"kubernetes.azure.com/use-osm-mtls":                 "true",
				"nginx.ingress.kubernetes.io/backend-protocol":      "HTTPS",
				"nginx.ingress.kubernetes.io/configuration-snippet": "\nproxy_ssl_name \"default.test-ns.cluster.local\";",
				"nginx.ingress.kubernetes.io/proxy-ssl-secret":      "kube-system/osm-ingress-client-cert",
				"nginx.ingress.kubernetes.io/proxy-ssl-verify":      "on",
			},
		},
		Spec: netv1.IngressSpec{
			IngressClassName: util.StringPtr(icName),
			Rules: []netv1.IngressRule{{
				Host: "test-host",
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pt,
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: svc.Name,
									Port: netv1.ServiceBackendPort{Number: svc.Spec.Ports[0].TargetPort.IntVal},
								},
							},
						}},
					},
				},
			}},
			TLS: []netv1.IngressTLS{{
				Hosts:      []string{"test-host"},
				SecretName: "keyvault-test-service",
			}},
		},
	}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(ing), ing))
	assert.Equal(t, expected, ing)

	// Override the default service account name
	svc.Annotations["kubernetes.azure.com/service-account-name"] = "test-sa"
	require.NoError(t, c.Update(ctx, svc))

	beforeErrCount = testutils.GetErrMetricCount(t, ingressControllerName)
	beforeReconcileCount = testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess)
	_, err = p.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, testutils.GetErrMetricCount(t, ingressControllerName), beforeErrCount)
	assert.Greater(t, testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess), beforeReconcileCount)

	expected.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"] = "\nproxy_ssl_name \"test-sa.test-ns.cluster.local\";"
	expected.ResourceVersion = "2"
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(ing), ing))
	assert.Equal(t, expected, ing)
}

func TestIngressReconcilerIntegrationNoOSM(t *testing.T) {
	svc := &corev1.Service{}
	svc.UID = "test-svc-uid"
	svc.Name = "test-service"
	svc.Namespace = "test-ns"
	svc.Annotations = map[string]string{
		"kubernetes.azure.com/ingress-host":          "test-host",
		"kubernetes.azure.com/tls-cert-keyvault-uri": "test-cert-uri",
		"kubernetes.azure.com/insecure-disable-osm":  "true",
	}
	svc.Spec.Ports = []corev1.ServicePort{{
		Port:       123,
		TargetPort: intstr.FromInt(234),
	}}

	c := fake.NewClientBuilder().WithObjects(svc).Build()
	p := &NginxIngressReconciler{
		client:    c,
		ingConfig: &manifests.NginxIngressConfig{IcName: icName},
	}

	ctx := context.Background()
	ctx = logr.NewContext(ctx, logr.Discard())

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}}
	beforeErrCount := testutils.GetErrMetricCount(t, ingressControllerName)
	beforeReconcileCount := testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess)
	_, err := p.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, testutils.GetErrMetricCount(t, ingressControllerName), beforeErrCount)
	assert.Greater(t, testutils.GetReconcileMetricCount(t, ingressControllerName, metrics.LabelSuccess), beforeReconcileCount)

	pt := netv1.PathTypePrefix
	expected := &netv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            svc.Name,
			Namespace:       svc.Namespace,
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Controller: util.BoolPtr(true),
				Kind:       "Service",
				Name:       svc.Name,
				UID:        svc.UID,
			}},
			Annotations: map[string]string{
				"kubernetes.azure.com/tls-cert-keyvault-uri": "test-cert-uri",
			},
		},
		Spec: netv1.IngressSpec{
			IngressClassName: util.StringPtr(icName),
			Rules: []netv1.IngressRule{{
				Host: "test-host",
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pt,
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: svc.Name,
									Port: netv1.ServiceBackendPort{Number: svc.Spec.Ports[0].TargetPort.IntVal},
								},
							},
						}},
					},
				},
			}},
			TLS: []netv1.IngressTLS{{
				Hosts:      []string{"test-host"},
				SecretName: "keyvault-test-service",
			}},
		},
	}
	ing := &netv1.Ingress{}
	ing.Name = svc.Name
	ing.Namespace = svc.Namespace
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(ing), ing))
	assert.Equal(t, expected, ing)
}
