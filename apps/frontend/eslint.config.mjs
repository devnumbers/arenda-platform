import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import jsxA11y from "eslint-plugin-jsx-a11y";
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

// XSS-class browser APIs banned outright (quality bar wave A, bar #330):
// HTML injection sinks and dynamic code evaluation. Zero usages today — the
// gate exists so a future regression is a lint error, not a review call.
const DANGEROUS_BROWSER_API_MESSAGE =
  "This browser API is an XSS-class sink (raw HTML injection or dynamic code evaluation) and is banned — render declarative React content instead (quality bar wave A, see apps/frontend/CODING_STANDARDS.md).";

// The security contour's no-restricted-syntax selectors (decision #331 +
// wave A), shared verbatim by the two blocks at the end of the config: the
// general block and the money-allowlist block that re-declares them for the
// two formatting modules. Sharing one array keeps the re-declaration from
// drifting — the allowlist may narrow the money selectors away, never the
// security ones.
const SECURITY_RESTRICTED_SYNTAX = [
  {
    selector: "JSXAttribute[name.name='urlTransform']",
    message:
      "react-markdown must keep its secure default urlTransform — overriding or disabling it (urlTransform={null}) reopens XSS vectors (decision #331).",
  },
  ...publicEnvSelectors(),
  // Wave A: XSS-class sinks — raw HTML injection and dynamic code
  // evaluation (0 usages today; the gate makes a regression impossible).
  // eval/Function/document.write are also matched through their
  // window./globalThis. alias forms, mirroring the web-storage block.
  {
    selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector: "MemberExpression[property.name='innerHTML']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector: "CallExpression[callee.property.name='insertAdjacentHTML']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector:
      "MemberExpression[object.name='document'][property.name=/^(write|writeln)$/]",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector:
      "MemberExpression[object.object.name=/^(window|globalThis)$/][object.property.name='document'][property.name=/^(write|writeln)$/]",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector: "CallExpression[callee.name='eval']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector:
      "CallExpression[callee.object.name=/^(window|globalThis)$/][callee.property.name='eval']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector: "NewExpression[callee.name='Function']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
  {
    selector:
      "NewExpression[callee.object.name=/^(window|globalThis)$/][callee.property.name='Function']",
    message: DANGEROUS_BROWSER_API_MESSAGE,
  },
];

// Money gate (quality bar wave C, bar #330, ticket #403): raw kopecks↔rubles
// arithmetic and .toFixed formatting are banned outside the formatting
// modules. Money crosses the boundary only through the canonical helpers in
// shared/lib/format-money.ts; percent widths scale via ratioToPercent. Both
// sides of /100 and *100 are banned — `100 * x` is the same arithmetic as
// `x * 100`.
const MONEY_ARITHMETIC_MESSAGE =
  "Raw kopecks↔rubles arithmetic (/100, *100) is banned (quality bar wave C, bar #330): convert through shared/lib/format-money.ts — kopecksToRublesString / parseRublesToKopecks for money, ratioToPercent for percent scaling.";

const MONEY_TO_FIXED_MESSAGE =
  ".toFixed lives only in the formatting modules (shared/lib/format-money.ts, features/property-attributes/lib/format.ts) — quality bar wave C, bar #330: money goes through the format-money helpers, and a new decimal format belongs in the owning format module (extending the allowlist is a deliberate config edit registered in docs/agents/tooling.md).";

const MONEY_FORMATTING_ALLOWLIST = [
  "shared/lib/format-money.ts",
  "features/property-attributes/lib/format.ts",
];

const MONEY_RESTRICTED_SYNTAX = [
  {
    selector: "BinaryExpression[operator='/'][right.value=100]",
    message: MONEY_ARITHMETIC_MESSAGE,
  },
  {
    selector: "BinaryExpression[operator='/'][left.value=100]",
    message: MONEY_ARITHMETIC_MESSAGE,
  },
  {
    selector: "BinaryExpression[operator='*'][right.value=100]",
    message: MONEY_ARITHMETIC_MESSAGE,
  },
  {
    selector: "BinaryExpression[operator='*'][left.value=100]",
    message: MONEY_ARITHMETIC_MESSAGE,
  },
  {
    selector: "MemberExpression[property.name='toFixed']",
    message: MONEY_TO_FIXED_MESSAGE,
  },
];

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
  // Quality bar wave A (bar #330, ticket #390) — configuration flips.
  // exhaustive-deps was warn via the next preset; 0 violations and 0
  // suppressions in the tree, so the flip is free. consistent-type-imports
  // lands together with tsconfig verbatimModuleSyntax — `import type` is the
  // only legal spelling for type-only imports.
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    rules: {
      "react-hooks/exhaustive-deps": "error",
      "@typescript-eslint/consistent-type-imports": "error",
    },
  },
  // The type-checked block (projectService). Wave A brought
  // switch-exhaustiveness-check; wave B (tickets #393, #394, #395 and #396,
  // bar #330) added the type-checked families fix-then-flip: every family was
  // counted advisory, fixed to zero, then flipped to error in the same change.
  // Scoped exactly to tsconfig.json's include extensions — a linted file
  // outside the project would fail to resolve its types.
  {
    files: ["**/*.{ts,mts,tsx}"],
    languageOptions: {
      parserOptions: {
        projectService: true,
      },
    },
    rules: {
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-misused-promises": "error",
      "@typescript-eslint/no-deprecated": "error",
      "@typescript-eslint/switch-exhaustiveness-check": "error",
      "@typescript-eslint/no-unnecessary-type-assertion": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
      "@typescript-eslint/no-base-to-string": "error",
      "@typescript-eslint/require-await": "error",
      // Wave C (bar #330, ticket #404): dead conditionals are compile-time
      // lies about the types beneath them.
      "@typescript-eslint/no-unnecessary-condition": "error",
      // Wave C (bar #330, ticket #405): `||` only where nullish is meant;
      // "empty string means absent" sites spell the check explicitly.
      "@typescript-eslint/prefer-nullish-coalescing": "error",
      // React-friendly options treat config-level noise with options, not
      // suppressions: arrow shorthand `() => void discard()` is the canonical
      // promise-discard spelling, and number interpolation in templates is
      // idiomatic UI code.
      "@typescript-eslint/no-confusing-void-expression": [
        "error",
        { ignoreArrowShorthand: true },
      ],
      "@typescript-eslint/restrict-template-expressions": [
        "error",
        { allowNumber: true },
      ],
    },
  },
  // Quality bar wave B (bar #330, ticket #397) — jsx-a11y `recommended` at
  // error: keyboard operability of custom controls, valid anchors, and no
  // `autoFocus` (programmatic focus instead). The plugin ships with
  // eslint-config-next and its preset already registers it, so only the
  // preset's rules land here — re-declaring the plugin object would be a
  // config error. alt-text keeps eslint-config-next's next/image scope
  // (`img: ["Image"]`) unioned with the preset's default element set.
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    rules: {
      ...jsxA11y.flatConfigs.recommended.rules,
      "jsx-a11y/alt-text": [
        "error",
        {
          elements: ["img", "object", "area", 'input[type="image"]'],
          img: ["Image"],
        },
      ],
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
  // Security contour (decision #331, ticket #387) + dangerous-browser-API bans
  // (quality bar wave A, bar #330, ticket #390) + the money gate (quality bar
  // wave C, bar #330, ticket #403). react-markdown must stay secure by
  // default: raw HTML (`rehype-raw`) is banned at the import level above, and
  // the `urlTransform` URL sanitizer (default: protocol filtering) may not be
  // overridden or disabled. The two blocks below are the config's last word
  // on no-restricted-syntax, so no files-scoped override can weaken the
  // security contour; the second one re-declares the security selectors (from
  // the shared SECURITY_RESTRICTED_SYNTAX array) for the money-allowlist
  // files, so ignoring those files for the money selectors never drops their
  // security coverage. (The web-storage block after these declares other
  // rules — no-restricted-globals/properties — and replaces nothing here.)
  {
    files: ["**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}"],
    ignores: MONEY_FORMATTING_ALLOWLIST,
    rules: {
      "no-restricted-syntax": [
        "error",
        ...SECURITY_RESTRICTED_SYNTAX,
        ...MONEY_RESTRICTED_SYNTAX,
      ],
    },
  },
  {
    files: MONEY_FORMATTING_ALLOWLIST,
    rules: {
      "no-restricted-syntax": [
        "error",
        ...SECURITY_RESTRICTED_SYNTAX,
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
