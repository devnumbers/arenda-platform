package wire

import (
	popupspg "github.com/nambers/arenda-planform/apps/backend/internal/popups/adapters/postgres"
	popupsapp "github.com/nambers/arenda-planform/apps/backend/internal/popups/application"
)

// Popups holds the popups module's service wired by WirePopups.
type Popups struct {
	Service *popupsapp.PopupService
}

// WirePopups constructs the popup repository and service.
func WirePopups(p platformDeps) *Popups {
	service := popupsapp.NewPopupService(popupspg.NewPopupRepository(p.DB))
	return &Popups{Service: service}
}
