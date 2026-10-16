// envOverrideBlock renders the exact override block appended to a worktree's
// .env (docs/agents/parallel-dev.md «Per-worktree .env»). The expected values
// below are the contract literals, not recomputations of slotConfig.
import { describe, expect, it } from "vitest";
import { envOverrideBlock, parseEnvFile, slotConfig } from "./lib.mjs";

describe("envOverrideBlock", () => {
  it("renders the contract block for slot 1 over a standard base env", () => {
    const block = envOverrideBlock(slotConfig(1), parseEnvFile("POSTGRES_USER=arenda\nPOSTGRES_PASSWORD=arenda\nPOSTGRES_DB=arenda\n"));
    const lines = block.split("\n").filter((l) => l && !l.startsWith("#"));
    expect(lines).toEqual([
      "AREND_SLOT=1",
      "POSTGRES_PORT=5441",
      "DATABASE_URL=postgres://arenda:arenda@localhost:5441/arenda?sslmode=disable",
      "HTTP_ADDR=:8091",
      "APP_BASE_URL=http://localhost:8091",
      "BACKEND_URL=http://localhost:8091",
      "COMPOSE_PROJECT_NAME=arenda-wt1",
      "E2E_PG_PORT=5446",
      "E2E_BACKEND_PORT=8096",
      "E2E_FRONTEND_PORT=3026",
      "E2E_COMPOSE_PROJECT=arenda-e2e-wt1",
    ]);
  });

  it("derives DATABASE_URL credentials from the copied env, not hardcoded arenda", () => {
    const block = envOverrideBlock(
      slotConfig(2),
      parseEnvFile("POSTGRES_USER=ivan\nPOSTGRES_PASSWORD=secret pass\nPOSTGRES_DB=platform\n"),
    );
    expect(block).toContain(
      "DATABASE_URL=postgres://ivan:secret%20pass@localhost:5442/platform?sslmode=disable",
    );
  });

  it("defaults the credentials to arenda when the base env omits POSTGRES_*", () => {
    const block = envOverrideBlock(slotConfig(9), parseEnvFile(""));
    expect(block).toContain(
      "DATABASE_URL=postgres://arenda:arenda@localhost:5449/arenda?sslmode=disable",
    );
  });
});
