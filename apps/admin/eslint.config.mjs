import { defineConfig, globalIgnores } from "eslint/config";
import tseslint from "typescript-eslint";

// Минимальный конфиг волны 2C (решение #295, тикет #307): только правила,
// переведённые из прозы apps/admin/AGENTS.md в инструментальный гейт.
// Полные пресеты (tseslint recommended и т.п.) сознательно не включаются —
// каждое новое правило отдельным решением.

export default defineConfig([
  globalIgnores(["dist/**"]),
  {
    files: ["src/**/*.{ts,tsx}"],
    languageOptions: { parser: tseslint.parser },
    plugins: { "@typescript-eslint": tseslint.plugin },
    rules: {
      // AGENTS.md «TypeScript: Avoid any; prefer unknown with narrowing».
      "@typescript-eslint/no-explicit-any": "error",
      // AGENTS.md «Runtime configuration comes from Vite env vars …; never
      // hardcode backend URLs»: абсолютные URL и литералы с зашитым
      // префиксом /api/ запрещены; путь запроса строится от API_PREFIX
      // (import.meta.env.VITE_API_PREFIX с фолбэком '/api').
      "no-restricted-syntax": [
        "error",
        {
          selector: "Literal[value=/^https?:\\/\\//]",
          message: "Backend URLs come from Vite env (VITE_API_PREFIX), never from hardcoded absolute-URL literals.",
        },
        {
          selector: "Literal[value=/^\\/api\\//]",
          message: "Backend URL paths are built from import.meta.env.VITE_API_PREFIX (API_PREFIX), not hardcoded '/api/...' literals.",
        },
      ],
    },
  },
  {
    // AGENTS.md «артефакты src/lib/generated/ не редактируются руками»:
    // generated-код импортируется только через шов propertyAttributes.
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/lib/propertyAttributes.ts"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["**/generated/**"],
              message:
                "Generated artifacts (src/lib/generated) are imported only through the src/lib/propertyAttributes.ts seam — import from './lib/propertyAttributes' instead.",
            },
          ],
        },
      ],
    },
  },
  {
    // AGENTS.md «route all backend calls through dataProvider and all auth
    // state through authProvider»: сырой fetch живёт только в трёх
    // санционированных HTTP-границах — данные (dataProvider), аутентификация
    // (authProvider) и телеметрия клиентских ошибок (report-error —
    // fire-and-forget с keepalive, сознательно вне dataProvider).
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/dataProvider.ts", "src/authProvider.ts", "src/lib/report-error.ts"],
    rules: {
      "no-restricted-globals": [
        "error",
        {
          name: "fetch",
          message:
            "Backend calls go through dataProvider (useDataProvider) or authProvider; raw fetch lives only in the sanctioned HTTP boundary files.",
        },
      ],
    },
  },
]);
