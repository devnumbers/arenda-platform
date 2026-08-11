package wire

import (
	adminpg "github.com/nambers/arenda-planform/apps/backend/internal/admin/adapters/postgres"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
)

// Admin holds the admin module's repository and service wired by WireAdmin.
type Admin struct {
	Repo    *adminpg.AdminRepository
	Service *adminapp.AdminService
}

// WireAdmin constructs the admin repository (which doubles as several ports)
// and the admin service. It takes the billing subscriptions port (used to read
// subscription state in admin views) and the occupancy provider (properties
// module).
func WireAdmin(p platformDeps, subscriptions application.Subscriber, occupancyProvider *propertiespg.OccupancyProvider) *Admin {
	repo := adminpg.NewAdminRepository(p.DB, p.Encryptor, p.Clock, occupancyProvider)
	service := adminapp.NewAdminService(repo, repo, repo, repo, repo, repo, subscriptions, repo, repo, p.Clock)
	return &Admin{Repo: repo, Service: service}
}
