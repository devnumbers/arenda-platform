import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import boundaries from "eslint-plugin-boundaries";

// FSD layers (app → widgets → features → entities → shared). Slices are the
// second path segment of widgets/features/entities; a file belongs to the
// slice folder it lives in, so same-slice imports stay internal while
// cross-slice imports are banned at every layer above shared.
const FSD_ELEMENTS = [
  // partialMatch:false — паттерны матчатся от корня проекта, а не любым суффиксом
  // пути (иначе будущий `features/x/shared/` или `shared/lib/app/` был бы
  // переклассифицирован как элемент shared/app).
  { type: "app", pattern: "app", partialMatch: false },
  { type: "widget", pattern: "widgets/*", capture: ["slice"], partialMatch: false },
  { type: "feature", pattern: "features/*", capture: ["slice"], partialMatch: false },
  { type: "entity", pattern: "entities/*", capture: ["slice"], partialMatch: false },
  { type: "shared", pattern: "shared", partialMatch: false },
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
          policies: [
            ...FSD_POLICIES,
            // Публичный API слайса (после FSD-политик — last-write-wins):
            // извне widgets/features/entities импортируется только index.ts.
            // Внутренние импорты слайса не проверяются (checkInternals=false);
            // shared — слой файловых модулей без ограничения точек входа.
            {
              disallow: {
                to: {
                  element: {
                    types: { anyOf: ["widget", "feature", "entity"] },
                    fileInternalPath: "!index.ts",
                  },
                },
              },
              message:
                "FSD: слайс импортируется извне только через свой публичный API — index.ts слайса",
            },
          ],
        },
      ],
      // Imports of local files matching no element pattern are reported, so a
      // new top-level directory can't silently escape the gate. Unresolvable
      // alias imports (typos) are reported by import/no-unresolved below.
      "boundaries/no-unknown-dependencies": "error",
      // Нерезолвимый alias-импорт (`@/typo/...`) boundaries пропускает молча;
      // это правило ловит такие value-импорты. Type-импорты (`import type`)
      // плагин игнорирует by design — их ловит tsc (`npm run build` в CI).
      "import/no-unresolved": "error",
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
  // Виджеты и страницы не знают о DTO: данные приходят им маплеными в
  // entity-модели из entities/features. Files-scoped правило перекрывает
  // базовое no-restricted-imports для этих слоёв, поэтому паттерны
  // generated/@heroui продублированы здесь.
  {
    files: ["widgets/**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}", "app/**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/shared/api/dto", "@/shared/api/dto.*", "**/shared/api/dto*", "@/shared/api/dto/**"],
              message:
                "DTO must not leak into widgets/app — consume entity models from entities/features; DTO mapping lives in their hooks and mappers.",
            },
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
