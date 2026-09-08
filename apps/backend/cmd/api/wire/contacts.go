package wire

import (
	contactspg "github.com/nambers/arenda-planform/apps/backend/internal/contacts/adapters/postgres"
	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
)

// Contacts holds the contacts module's service wired by WireContacts; the
// HTTP adapters of the contact book live in
// internal/contacts/adapters/http and mount the service in the composition
// root.
type Contacts struct {
	ContactService *contactsapp.ContactService
}

// WireContacts constructs the contacts context (ADR 0054): the contact book
// store, the property reference store, the shared transactional factory
// (ADR 0033 γ-factory) and the use case service. The policy comes from the
// access module — contacts is wired after it, so the membership-aware policy
// is already resolved.
func WireContacts(p platformDeps) (*Contacts, error) {
	contactStore := contactspg.NewContactStore(p.DB)
	propertyStore := contactspg.NewPropertyStore(p.DB)

	factory := contactsapp.NewTxStoreFactory(
		contactStore,
		propertyStore,
		p.AuditRecorder,
		p.UoW,
	)

	return &Contacts{
		ContactService: contactsapp.NewContactService(factory, p.Policy),
	}, nil
}
