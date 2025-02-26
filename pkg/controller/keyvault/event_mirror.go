// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package keyvault

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Azure/aks-app-routing-operator/pkg/config"
	"github.com/Azure/aks-app-routing-operator/pkg/controller/metrics"
	"github.com/Azure/aks-app-routing-operator/pkg/util"
)

const (
	eventMirrorControllerName = "event_mirror"
)

// EventMirror copies events published to pod resources by the Keyvault CSI driver into ingress events.
// This allows users to easily determine why a certificate might be missing for a given ingress.
type EventMirror struct {
	client client.Client
	events record.EventRecorder
}

func NewEventMirror(manager ctrl.Manager, conf *config.Config) error {
	metrics.InitControllerMetrics(eventMirrorControllerName)
	if conf.DisableKeyvault {
		return nil
	}
	e := &EventMirror{
		client: manager.GetClient(),
		events: manager.GetEventRecorderFor("aks-app-routing-operator"),
	}
	return ctrl.
		NewControllerManagedBy(manager).
		For(&corev1.Event{}).
		WithEventFilter(e.newPredicates()).
		Complete(e)
}

func (e *EventMirror) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	result := ctrl.Result{}

	// do metrics
	defer func() {
		//placing this call inside a closure allows for result and err to be bound after Reconcile executes
		//this makes sure they have the proper value
		//just calling defer metrics.HandleControllerReconcileMetrics(controllerName, result, err) would bind
		//the values of result and err to their zero values, since they were just instantiated
		metrics.HandleControllerReconcileMetrics(eventMirrorControllerName, result, err)
	}()

	logger, err := logr.FromContext(ctx)
	if err != nil {
		return result, err
	}
	logger = logger.WithName("eventMirror")

	event := &corev1.Event{}
	err = e.client.Get(ctx, req.NamespacedName, event)
	if err != nil {
		return result, client.IgnoreNotFound(err)
	}

	// Filter to include only keyvault mounting errors
	if event.InvolvedObject.Kind != "Pod" ||
		event.Reason != "FailedMount" ||
		!strings.HasPrefix(event.InvolvedObject.Name, "keyvault-") ||
		!strings.Contains(event.Message, "keyvault") {
		return result, nil
	}

	// Get the owner (pod)
	pod := &corev1.Pod{}
	pod.Name = event.InvolvedObject.Name
	pod.Namespace = event.InvolvedObject.Namespace
	err = e.client.Get(ctx, client.ObjectKeyFromObject(pod), pod)
	if err != nil {
		return result, client.IgnoreNotFound(err)
	}
	if pod.Annotations == nil {
		return result, nil
	}

	// Get the owner (ingress)
	ingress := &netv1.Ingress{}
	ingress.Namespace = pod.Namespace
	ingress.Name = pod.Annotations["kubernetes.azure.com/ingress-owner"]
	err = e.client.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)
	if err != nil {
		return result, client.IgnoreNotFound(err)
	}

	// Publish to the service also if ingress is owned by a service
	if name := util.FindOwnerKind(ingress.OwnerReferences, "Service"); name != "" {
		svc := &corev1.Service{}
		svc.Namespace = pod.Namespace
		svc.Name = name
		err = e.client.Get(ctx, client.ObjectKeyFromObject(svc), svc)
		if err != nil {
			return result, client.IgnoreNotFound(err)
		}
		e.events.Event(svc, "Warning", "FailedMount", event.Message)
	}

	e.events.Event(ingress, "Warning", "FailedMount", event.Message)
	return result, nil
}

func (e *EventMirror) newPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
