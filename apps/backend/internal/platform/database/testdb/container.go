//go:build integration

package testdb

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// postgresImage matches the PostgreSQL version used in local, stage, and prod
// (docker-compose.*.yml). Keeping the test container on the same major version
// avoids migration/syntax drift between test and production.
const postgresImage = "postgres:18-alpine"

// container holds the testcontainers handle so it can be terminated when the
// test binary exits (see [terminateContainer]). Protected by containerMu
// because startPostgres runs inside [sync.OnceValue] (single-writer) but
// terminateContainer runs from TestMain after all tests — a data-race fence
// is cheap insurance for the shared package-level pointer.
var (
	container   *postgres.PostgresContainer
	containerMu sync.Mutex
)

// startPostgres starts a PostgreSQL container via testcontainers-go and returns
// its connection string with sslmode=disable. The container is kept alive for
// the lifetime of the test binary and terminated via [terminateContainer],
// which is wired up in TestMain.
//
// A failure to start the container is fatal for the entire test binary, so the
// function panics; [sync.OnceValue] propagates the panic to the first caller
// of databaseURL, which surfaces it as a test failure.
func startPostgres() string {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, postgresImage,
		postgres.WithDatabase("arenda"),
		postgres.WithUsername("arenda"),
		postgres.WithPassword("arenda"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic(fmt.Errorf("testdb: start postgres container: %w", err))
	}

	containerMu.Lock()
	container = ctr
	containerMu.Unlock()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Errorf("testdb: container connection string: %w", err))
	}

	return connStr
}

// terminateContainer stops and removes the PostgreSQL container if one was
// started. It is a no-op when an external TEST_DATABASE_URL was used. Call this
// from TestMain after m.Run() returns.
func terminateContainer() {
	containerMu.Lock()
	ctr := container
	containerMu.Unlock()

	if ctr == nil {
		return
	}
	if err := testcontainers.TerminateContainer(ctr); err != nil {
		// A teardown error is non-fatal: the container will be reaped by
		// Ryuk (testcontainers' cleanup sidecar) or Docker garbage collection.
		_, _ = fmt.Fprintf(os.Stderr, "testdb: terminate container: %v\n", err)
	}
}

// dockerAvailable returns true when a Docker daemon is likely reachable. It is
// used to produce a clear skip message instead of a cryptic testcontainers
// error when Docker is not running. The check is heuristic: testcontainers
// resolves the socket itself and may still succeed where this returns false.
func dockerAvailable() bool {
	if socket := os.Getenv("DOCKER_HOST"); socket != "" {
		return true
	}
	// Docker Desktop on macOS creates /var/run/docker.sock as a symlink;
	// on Linux it is the native socket. If neither exists, Docker is
	// very likely not running.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return true
	}
	return false
}
