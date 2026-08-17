import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import boundaries from "eslint-plugin-boundaries";

// FSD layers (app → widgets → features → entities → shared). Slices are the
// second path segment of widgets/features/entities; a file belongs to the
// slice folder it lives in, so same-slice imports stay internal while
// cross-slice imports are banned at every layer above shared.
const FSD_ELEMENTS = [
  { type: "app", pattern: "app" },
  { type: "widget", pattern: "widgets/*", capture: ["slice"] },
  { type: "feature", pattern: "features/*", capture: ["slice"] },
  { type: "entity", pattern: "entities/*", capture: ["slice"] },
  { type: "shared", pattern: "shared" },
];

const LOWER_THAN = {
  app: ["widget", "feature", "entity", "shared"],
  widget: ["feature", "entity", "shared"],
  feature: ["entity", "shared"],
  entity: ["shared"],
  shared: [],
};

// One policy per layer: imports may go one layer down the hierarchy and stay
// inside the same slice; everything else (upward, skipping layers, cross-slice)
// is disallowed by the rule-level default.
const FSD_POLICIES = Object.entries(LOWER_THAN).map(([layer, lowerLayers]) => ({
  from: { element: { type: layer } },
  allow: [
    ...lowerLayers.map((target) => ({
      to: { element: { types: { anyOf: [target] } } },
    })),
    {
      to: {
        element: {
          type: layer,
          captured: { slice: "{{ from.element.captured.slice }}" },
        },
      },
    },
  ],
  message: `FSD: ${layer} may import only lower layers (${lowerLayers.length > 0 ? lowerLayers.join(", ") : "none"}) and its own slice`,
}));

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  {
    plugins: { boundaries },
    settings: {
      "boundaries/elements": FSD_ELEMENTS,
      "import/resolver": {
        typescript: { alwaysTryTypes: true },
      },
    },
    rules: {
      "boundaries/dependencies": [
        "error",
        {
          default: "disallow",
          policies: FSD_POLICIES,
        },
      ],
      // Imports of local files matching no element pattern are reported, so a
      // new top-level directory can't silently escape the gate. Unresolvable
      // alias imports are skipped by the plugin — tsc/CI build catch those.
      "boundaries/no-unknown-dependencies": "error",
    },
  },
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    ignores: ["shared/api/**"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/shared/api/generated", "@/shared/api/generated.*", "**/shared/api/generated*"],
              message:
                "The generated API client is imported only inside shared/api — import DTO types from @/shared/api/dto instead.",
            },
            {
              group: ["@heroui/styles", "@heroui/styles/*"],
              message:
                "@heroui/styles (BEM classes) must not be imported in React components — use @heroui/react components (HeroUI v3 boundary).",
            },
          ],
        },
      ],
    },
  },
]);

export default eslintConfig;
