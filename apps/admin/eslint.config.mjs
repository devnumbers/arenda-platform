import { defineConfig, globalIgnores } from "eslint/config";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";

// Минимальный конфиг волны 2C (решение #295, тикет #307): только правила,
// переведённые из прозы apps/admin/AGENTS.md в инструментальный гейт.
// Полные пресеты (tseslint recommended и т.п.) сознательно не включаются —
// каждое новое правило отдельным решением.
//
// Исключение — админ-трек I бара качества (бар #330, спека #378, тикет #391):
// recommendedTypeChecked берётся целиком по измеренному сигналу/шуму
// (research #328 §6) вместе с react-hooks recommended — единственной новой
// devDependency всего режима качества.

const URL_LITERAL_SELECTORS = [
  {
    selector: "Literal[value=/^https?:\\/\\//]",
    message:
      "Backend URLs come from Vite env (VITE_API_PREFIX), never from hardcoded absolute-URL literals.",
  },
  {
    selector: "Literal[value=/^\\/api\\//]",
    message:
      "Backend URL paths are built from import.meta.env.VITE_API_PREFIX (API_PREFIX), not hardcoded '/api/...' literals.",
  },
];

// Деньги-гейт (бар #330, админ-трек I): сырая арифметика `/100`/`*100` и
// `.toFixed` вне src/fields.tsx — деньги ходят целыми копейками и форматируются
// только через MoneyField/formatKopecks. Литерал 100 матчится по значению с
// обеих сторон выражения (спека #378); не-денежные конверсии с другим
// множителем (мс→дни в lib/subscription — 24*60*60*1000) селектором не
// ловятся.
const MONEY_SELECTORS = [
  {
    selector:
      "BinaryExpression[operator='/'][left.value=100], BinaryExpression[operator='/'][right.value=100]",
    message:
      "Raw kopecks arithmetic (/100) is banned outside src/fields.tsx — money is integer kopecks, formatted only via MoneyField/formatKopecks.",
  },
  {
    selector:
      "BinaryExpression[operator='*'][left.value=100], BinaryExpression[operator='*'][right.value=100]",
    message:
      "Raw kopecks arithmetic (*100) is banned outside src/fields.tsx — money is integer kopecks, formatted only via MoneyField/formatKopecks.",
  },
  {
    selector: "CallExpression[callee.property.name='toFixed']",
    message:
      ".toFixed is banned outside src/fields.tsx — money formatting goes through MoneyField/formatKopecks (integer kopecks).",
  },
];

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
      "no-restricted-syntax": ["error", ...URL_LITERAL_SELECTORS],
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
  // Админ-трек I бара качества (бар #330, тикет #391): type-checked пресет
  // на ручной код. HTTP-границы dataProvider/authProvider типизированы в
  // админ-треке II (#392) и живут под пресетом наравне с остальным ручным
  // кодом. Шаблон генератора property-attributes не правится под бар —
  // generated-каталог живёт под собственным files-scoped блоком без
  // type-checked правил.
  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/lib/generated/**"],
    extends: [tseslint.configs.recommendedTypeChecked],
    languageOptions: {
      parserOptions: {
        projectService: true,
      },
    },
    rules: {
      // Добор шага 2 research #328 поимённо (спека #378, админ-трек).
      "@typescript-eslint/prefer-nullish-coalescing": "error",
    },
  },
  {
    // react-hooks recommended — правила хуков статикой, а не падением
    // в рантайме; единственная новая devDependency режима качества.
    // configs.flat.* — объектная форма plugins (configs.recommended —
    // строковая, под ESLint 10). generated — files-scoped, как у пресета
    // и деньги-селекторов (плагин и так пропускает не-React файлы,
    // исключение фиксирует границу на будущее).
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/lib/generated/**"],
    extends: [reactHooks.configs.flat.recommended],
  },
  {
    // Деньги-селекторы вне денежного поля админки. Files-scoped блок с
    // игнором fields.tsx/generated: в flat-config последний блок, задавший
    // no-restricted-syntax для файла, ЗАМЕНЯЕТ правило целиком — селекторы
    // URL-литералов из базового блока повторены здесь (паттерн рестейта
    // волны 2E фронта, ловушка задокументирована в его конфиге).
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/fields.tsx", "src/lib/generated/**"],
    rules: {
      "no-restricted-syntax": ["error", ...URL_LITERAL_SELECTORS, ...MONEY_SELECTORS],
    },
  },
]);
