// Package fixtures loads and normalizes the perfvegeta fixture file.
package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nambers/arenda-planform/apps/backend/cmd/perfvegeta/model"
)

// Load reads and normalizes the fixtures file.
func Load(path string) (model.Fixtures, error) {
	// #nosec G304 -- path is supplied by the caller and cleaned before use.
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return model.Fixtures{}, err
	}

	var fx model.Fixtures
	if err := json.Unmarshal(data, &fx); err != nil {
		return model.Fixtures{}, err
	}

	fx.Normalize()
	if len(fx.Tokens) == 0 {
		return model.Fixtures{}, fmt.Errorf("fixtures contain no tokens")
	}
	return fx, nil
}
