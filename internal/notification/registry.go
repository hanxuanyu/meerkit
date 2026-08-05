package notification

import (
	"meerkit/internal/core"
	"meerkit/internal/notification/inapp"
	"meerkit/internal/notification/smtp"
	"meerkit/internal/notification/webhook"
	"meerkit/internal/store"
)

type Registry struct {
	notifiers map[string]core.NotifierModule
}

func NewRegistry(store *store.Store, hub *inapp.Hub) *Registry {
	registry := &Registry{notifiers: make(map[string]core.NotifierModule)}
	registry.Register(inapp.New(store, hub))
	registry.Register(webhook.New())
	registry.Register(smtp.New())
	return registry
}

func (r *Registry) Register(notifier core.NotifierModule) {
	r.notifiers[notifier.Descriptor().Type] = notifier
}
func (r *Registry) Get(notifierType string) (core.NotifierModule, bool) {
	notifier, ok := r.notifiers[notifierType]
	return notifier, ok
}
func (r *Registry) Descriptors() []core.NotifierDescriptor {
	result := make([]core.NotifierDescriptor, 0, len(r.notifiers))
	for _, notifier := range r.notifiers {
		result = append(result, notifier.Descriptor())
	}
	return result
}
