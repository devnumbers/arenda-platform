package domain

type TenantContactWithLeases struct {
	TenantContact
	ActiveLease *Lease
	LastLease   *Lease
}
