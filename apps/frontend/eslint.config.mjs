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

// Public env variables deliberately exposed to the client bundle (decision
// #331). Empty today: any NEXT_PUBLIC_* read in code is an error, so a new
// variable cannot leak into the client bundle silently — legalizing one is an
// explicit edit of this list in the same change that updates the registry in
// docs/agents/tooling.md.
const PUBLIC_ENV_ALLOWLIST = [];

// NEXT_PUBLIC_* selectors ban every read form not on the allowlist: member
// access (process.env.NEXT_PUBLIC_X), computed literal access
// (process.env["NEXT_PUBLIC_X"]) and destructuring from process.env.
function publicEnvSelectors() {
  const ban = PUBLIC_ENV_ALLOWLIST.length
    ? new RegExp(`^NEXT_PUBLIC_(?!${PUBLIC_ENV_ALLOWLIST.join("$|")}$)`)
    : /^NEXT_PUBLIC_/;
  const message =
    "NEXT_PUBLIC_* variables ship to the client bundle — exposure is a deliberate decision (decision #331): add the variable to PUBLIC_ENV_ALLOWLIST in eslint.config.mjs, not to the code.";
  return [
    {
      selector: `MemberExpression[object.object.name='process'][object.property.name='env'][property.name=/${ban.source}/]`,
      message,
    },
    {
      selector: `MemberExpression[object.object.name='process'][object.property.name='env'][property.value=/${ban.source}/]`,
      message,
    },
    {
      selector: `VariableDeclarator[init.object.name='process'][init.property.name='env'] Property[key.name=/${ban.source}/]`,
      message,
    },
  ];
}

// rehype-raw renders raw HTML from markdown, breaking react-markdown's
// secure-by-default pipeline (decision #331). One pattern object, restated in
// both no-restricted-imports blocks below: the files-scoped widgets/app block
// replaces (not merges) the base rule for those files.
const REHYPE_RAW_PATTERN = {
  group: ["rehype-raw"],
  message:
    "rehype-raw renders raw HTML from markdown, breaking react-markdown's secure-by-default pipeline (decision #331) — revisiting is an explicit decision, see docs/agents/tooling.md.",
};

const WEB_STORAGE_MESSAGE =
  "Web storage is owned by features/auth/lib and the shared draft store (shared/lib/hooks/useDraftStore) — tokens live in httpOnly cookies and must not spread into localStorage/sessionStorage (decision #331).";

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
            REHYPE_RAW_PATTERN,
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
// rehype-raw/generated/@heroui продублированы здесь.
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
            REHYPE_RAW_PATTERN,
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
  // Security contour (decision #331, ticket #387). react-markdown must stay
  // secure by default: raw HTML (`rehype-raw`) is banned at the import level
  // above, and the `urlTransform` URL sanitizer (default: protocol filtering)
  // may not be overridden or disabled. These blocks are the last in the config
  // so no files-scoped override can weaken them.
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXAttribute[name.name='urlTransform']",
          message:
            "react-markdown must keep its secure default urlTransform — overriding or disabling it (urlTransform={null}) reopens XSS vectors (decision #331).",
        },
        ...publicEnvSelectors(),
      ],
    },
  },
  // Web storage is owned by the auth module and the shared draft store only:
  // session tokens live in httpOnly cookies and must not spread into
  // localStorage/sessionStorage (decision #331; the whitelist paths hold the
  // login draft, the resend cooldown and useDraftStore — no secrets).
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    ignores: ["features/auth/lib/**", "shared/lib/hooks/useDraftStore.ts"],
    rules: {
      "no-restricted-globals": [
        "error",
        { name: "localStorage", message: WEB_STORAGE_MESSAGE },
        { name: "sessionStorage", message: WEB_STORAGE_MESSAGE },
      ],
      "no-restricted-properties": [
        "error",
        { object: "window", property: "localStorage", message: WEB_STORAGE_MESSAGE },
        { object: "window", property: "sessionStorage", message: WEB_STORAGE_MESSAGE },
        { object: "globalThis", property: "localStorage", message: WEB_STORAGE_MESSAGE },
        { object: "globalThis", property: "sessionStorage", message: WEB_STORAGE_MESSAGE },
      ],
    },
  },
]);

export default eslintConfig;
