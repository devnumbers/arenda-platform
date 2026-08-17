import { expect, test } from "vitest";

// PLANTED FAILURE — verification for #304 checkbox 2; reverted right after.
test("planted failure (304)", () => {
  expect(1).toBe(2);
});
