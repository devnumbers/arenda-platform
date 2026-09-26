// Прогон /pre-merge: весь гейт перед влитием в один воркфлоу —
// интеграция свежего dev → полный тест-бейзлайн → цикл (ревью + архитектура по областям →
// кластеры правок → быстрые гейты → коммиты) до нуля подтверждённых находок →
// финальный полный прогон → merge в dev в ГЛАВНОМ чекауте → отчёт в карту.
// Пуш не делается никогда; снос ворктри/стендов — вне прогона (по слову владельца).
// Перед запуском правится ТОЛЬКО блок КОНФИГА ниже; механика дальше — проверенная, не трогать.

// ===================== КОНФИГ ПРОГОНА (правит живая сессия) =====================
// Абсолютный путь к ворктри фичевой ветки.
const WT = "/Users/smirnowwwivan/Nambers/arenda-planform/.worktrees/BRANCH";
// Имя фичевой ветки.
const BRANCH = "BRANCH";
// Абсолютный путь главного чекаута (там живёт dev и там же происходит влитие).
const MAIN = "/Users/smirnowwwivan/Nambers/arenda-planform";
// Номер или URL карты усилия (спека + адрес отчёта). Пустая строка — спека и отчёт в тикет пропускаются.
const MAP_ISSUE = "";
// Области ревью. Пустой массив — механическая раскладка по директориям диффа.
// Ручная раскладка (рекомендуется): paths — префиксы путей, файлы области скрипт считает сам.
const AREAS: AreaDef[] = [
  // {
  //   id: "backend-access",
  //   title: "Бэкенд: access-агрегат и SQL",
  //   paths: ["apps/backend/internal/access", "apps/backend/db"],
  //   standards: "apps/backend/AGENTS.md, apps/backend/internal/access/CONTEXT.md, CONTEXT-MAP.md; деньги — BIGINT копейки, docs/adr/0036.",
  //   focus: "Что именно проверить в этой ветке: ключевые коммиты, швы, риски области.",
  // },
];
// Диспозиции прошлых раундов и пины владельца (НЕ находки, ревьюверам не переоткрывать).
const DISPO: string[] = [
  // "Активный грант доступа не публикует событие — запинено тестом …",
];
// Гнать e2e на слитом dev ПОСЛЕ влития (главный чекаут, disposable-стек слота 0 — безопасно;
// в ворктри e2e не гоняется никогда — грабля сноса чужих слотов). Медленно: +10–20 минут.
const E2E_AFTER_MERGE = false;
// Спрашивать владельца финальное слово перед самим merge, даже когда гейт чист (вопрос через эскалацию).
const ASK_BEFORE_MERGE = false;
// ===============================================================================

const SUITE_TIMEOUT_MS = 5_400_000; // полный make test с Docker
const GATE_TIMEOUT_MS = 900_000; // быстрые гейты (vitest/go)
const SWEEP_ROUNDS = 3; // потолок ревью-раундов; остаток = осознанный остаток, влития нет
const REPAIR_ROUNDS = 3; // потолок починки красных быстрых гейтов до коммитов
const BASELINE_REPAIRS = 2; // потолок починки красного бейзлайна

// --- Типы ---
interface AreaDef {
  /** Короткий латинский id области. */
  id: string;
  /** Человеческое название области, по-русски. */
  title: string;
  /** Префиксы путей области от корня репозитория ("." — весь репозиторий). */
  paths: string[];
  /** Файлы стандартов и канонов области (AGENTS.md, CONTEXT.md, DESIGN.md, ADR). */
  standards: string;
  /** Что именно проверить в этой ветке: швы, коммиты, риски. */
  focus: string;
}

interface RawFinding {
  /** Путь от корня репозитория с номером строки: "apps/backend/db/queries/participants.sql:42". */
  where: string;
  /** Одно предложение: что не так (или что предлагают углубить). */
  what: string;
  /** Доказательство: цитаты строк кода или читающая команда, которые это показывают. */
  evidence: string;
  /** high — потеря данных, крэш или реальная регрессия; medium — нарушение канона/контракта или сильный арх-кандидат; low — мелочь. */
  severity: "high" | "medium" | "low";
  /** defect — дефект или doc-рот; architecture — кандидат углубления; spec — расхождение со спекой. */
  kind: "defect" | "architecture" | "spec";
  /** Для architecture — форма углубления; для defect — короткая подсказка фикса (можно опустить). */
  fixHint?: string;
}

interface ReviewOutcome {
  /** 2–5 предложений: что именно проверено в области и каково общее состояние. */
  summary: string;
  /** Реальные находки; пустой список — нормальный и честный ответ. */
  findings: RawFinding[];
}

interface Confirmation {
  /** Подтверждено — находка воспроизводится; Опровергнуто — ошибочна; Неясно — не удалось ни то ни другое. */
  verdict: "Подтверждено" | "Опровергнуто" | "Неясно";
  /** Одно-два предложения: почему такой вердикт, со ссылками путь:строка. */
  reason: string;
}

interface JudgedFinding extends RawFinding {
  area: string;
  verdict: Confirmation["verdict"];
  confirmReason: string;
}

interface DisposedFinding extends JudgedFinding {
  /** Порядковый номер находки в раунде — им триажёр ссылается на неё. */
  id: number;
  round: number;
}

interface AreaResult {
  title: string;
  summary: string;
  findings: JudgedFinding[];
}

interface Cluster {
  id: string;
  title: string;
  /** Порядок коммита внутри раунда. */
  order: number;
  /** Конкретные пути файлов кластера от корня репозитория; кластеры не пересекаются по файлам. */
  files: string[];
  /** Инструкции фиксировщику по каждой находке кластера. */
  task: string;
  /** Готовая первая строка коммита в каноне репозитория. */
  commitMsg: string;
  /** id находок (из JSON триажёру), которые закрывает кластер. */
  findingIds: number[];
}

interface TriageOutcome {
  clusters: Cluster[];
  /** Находки, которые решено не чинить: причина обязательна, попадут в отчёт и диспозиции. */
  declines: Array<{ findingId: number; reason: string }>;
  /** Кандидаты за пределами бласт-радиуса ветки: оформить issue. */
  issuesToFile: Array<{ findingId: number; title: string; body: string }>;
  /** Как разошлись неясные находки и что решено с владельцем. */
  note: string;
}

interface FixResult {
  /** Что сделано по каждой находке кластера, 2–6 предложений. */
  summary: string;
  /** Все изменённые и созданные файлы, пути от корня репозитория. */
  changedFiles: string[];
  /** Какие тесты гоняли и чем кончилось (команда → результат). */
  testsRun: string;
}

interface CommitResult {
  /** Хэш созданного коммита, или "нет изменений", если добавлять было нечего. */
  hash: string;
  /** Первая строка сообщения созданного коммита. */
  subject: string;
}

interface GateFailure {
  gate: string;
  output: string;
}

interface SuiteResult {
  ok: boolean;
  how: string;
  failures: string[];
  /** Суиты, которые прогнать не удалось (вывод не влез в лимит) — честное not-covered. */
  unknown: string[];
}

interface GateReportItem {
  where: string;
  what: string;
  evidence: string;
  status: "verified" | "unconfirmed";
  severity: "high" | "medium" | "low";
}

// --- Персоны ---
const PERSONA_REVIEWER =
  "Ты старший ревьюер финального гейта перед влитием фичевой ветки в dev. Ты читаешь код придирчиво, " +
  "но не выдумываешь находки ради количества: пустой список находок — нормальный и честный ответ. " +
  "Каждую находку подкрепляй ссылкой путь:строка и цитатой. Ты НЕ редактируешь файлы, НЕ запускаешь " +
  "make/docker/e2e/тест-сьюты и не делаешь записывающих git-операций — только чтение, поиск и читающие git-команды. " +
  "Если задача невыполнима или инструкции противоречат друг другу — скажи об этом прямо в ответе.";

const PERSONA_SPEC_REVIEWER =
  "Ты ревьюер соответствия спеке: сверяешь дифф ветки с картой усилия и её тикетами. " +
  "Каждую находку подкрепляй цитатой спеки и ссылкой путь:строка. Пустой список — нормальный и честный ответ. " +
  "Ты НЕ редактируешь файлы, НЕ запускаешь тест-сьюты; gh — только читающие команды (issue view), git — только читающие команды.";

const PERSONA_CONFIRMER =
  "Ты независимый проверяющий: тебе дают находку ревьюера, ты воспроизводишь её с нуля по коду, " +
  "не доверяя формулировке автора находки. Ты НЕ редактируешь файлы и не запускаешь тест-сьюты — " +
  "только чтение, поиск и читающие git-команды. Вердикт обосновывай ссылками путь:строка. " +
  "Если невозможно ни воспроизвести, ни опровергнуть — честно отвечай «Неясно», не угадывай.";

const PERSONA_INTEGRATOR =
  "Ты разруливаешь конфликты merge dev в фичевую ветку перед гейтом. Ты правишь ТОЛЬКО конфликтные файлы " +
  "и завершаешь merge коммитом; тест-сьюты не запускаешь, новое поведение не изобретаешь, git merge --abort никогда. " +
  "Спорная развязка (что оставлять — решает владелец) — задай вопрос прямо: контекст, варианты, твоя рекомендация.";

const PERSONA_TRIAGE =
  "Ты старший инженер, раскладывающий подтверждённые находки ревью-гейта по кластерам правок перед влитием ветки. " +
  "Ты читаешь код, чтобы задачи были конкретными (путь:строка, что именно сделать), но сам ничего не правишь и не коммитишь. " +
  "Если фикс находки невозможен без смены поведения — сразу задай владельцу вопрос (эскалация): что именно менять, варианты с рекомендацией. " +
  "Не изобретай обходных путей и не молчи о развязках.";

const PERSONA_FIXER =
  "Ты старший фронтенд/бэкенд-инженер, закрываешь диспозиции ревью-гейта перед влитием ветки. " +
  "Правь минимально и в стиле окружающего кода; комментарии в коде — по-русски, как в соседних файлах, и никогда не упоминай ревью, агентов или этот прогон — комментарий объясняет код следующему читателю. " +
  "Ты работаешь ТОЛЬКО в файлах своего кластера (плюс новые тесты рядом с ними), НЕ делаешь git-коммитов, НЕ запускаешь e2e/docker/полные сьюты и не трогаешь сгенерированное (generated.ts, openapi, sqlc). " +
  "Находки линтеров исправляй КОДОМ: глушение (//nolint, eslint-disable, @ts-ignore) и обход pre-commit-хуков запрещены — если находка кажется ложной, скажи об этом прямо в ответе вместо глушения. " +
  "Целевые тесты гонять можно и нужно (vitest/go test по своим файлам); behavior-правки делай test-first: падающий тест → фикс → зелёный. " +
  "Если задача невыполнима или инструкции противоречат друг другу — скажи об этом прямо, не изобретай обходных путей.";

const PERSONA_REPAIRER =
  "Ты чинишь падения тестовых гейтов в ворктри ветки минимальными правками, не меняя смысл уже сделанных диспозиций. " +
  "Находки линтеров исправляй кодом, а не глушением (//nolint, eslint-disable запрещены как обход). " +
  "Не коммитишь, e2e/docker не трогаешь, сгенерированное (generated.ts, openapi, sqlc) руками не правишь. " +
  "Если падение нельзя починить без чужих файлов или оно противоречит задаче — скажи прямо.";

const PERSONA_COMMITTER =
  "Ты аккуратный коммитер: выполняешь ровно то, что сказано в задании — git add перечисленных путей и git commit с готовым сообщением в ворктри ветки. " +
  "Обход pre-commit-хуков ЗАПРЕЩЁН: ни --no-verify, ни LEFTHOOK=0. Если хук упал — коммит не делай, верни в ответе хвост вывода упавшего хука как есть: чинильщик получит его и исправит. " +
  "Ничего не правишь, ничего не пушишь, гейтов не гоняешь, лишних файлов не добавляешь.";

const PERSONA_EDITOR =
  "Ты редактор сообщений слияния. По истории merge-коммитов репозитория и сводке усилия составь ОДНУ строку сообщения в каноне репозитория " +
  "(начало — Merge branch '<ветка>' into dev — далее плотное описание усилия: карта, что сделано, чем закрыт гейт). " +
  "Ты только читаешь git-историю; верни ровно строку сообщения без кавычек и пояснений.";

const PERSONA_REPORTER =
  "Ты публикуешь отчёт гейта: размести текст КАК ЕСТЬ комментарием в issue через gh issue comment, затем закрой issue командой gh issue close. " +
  "Ничего не редактируй в тексте, ничего не добавляй от себя, пуша не делай. Верни URL созданного комментария.";

const PERSONA_GUARD =
  "Ты страж финального решения о влитии. Твоя единственная работа — задать владельцу один чёткий вопрос " +
  "(эскалация) с кратким контекстом и дождаться ответа. Ничего не читаешь, не правишь, не запускаешь. " +
  "Верни ровно то, что решил владелец: merge — если подтвердил; hold и причину одной строкой — если отложил.";

// --- Детерминированные помощники ---
async function gitOut(dir: string, gitArgs: string[]): Promise<string> {
  const r = await world.run("git", ["-C", dir, ...gitArgs]);
  if (r.exitCode !== 0) {
    throw new Error("git " + gitArgs.join(" ") + " в " + dir + " упал: " + (r.stderr || r.stdout).slice(0, 400));
  }
  return r.stdout.trim();
}

/** Полный прогон: make test-log (вывод в .make-test.log чекаута, stdout — только итог и хвост при падении); фолбэк — по целевым суитам. */
async function runFullSuite(dir: string, label: string): Promise<SuiteResult> {
  try {
    const r = await world.run("make", ["-C", dir, "test-log"], { timeoutMs: SUITE_TIMEOUT_MS });
    if (r.exitCode === 0) return { ok: true, how: label + ": make test-log exit 0 (полный сьют зелёный)", failures: [], unknown: [] };
    return {
      ok: false,
      how: label + ": make test-log exit " + r.exitCode + " (полный лог: " + dir + "/.make-test.log)",
      failures: [(r.stdout + "\n" + r.stderr).slice(-3000)],
      unknown: [],
    };
  } catch (capErr) {
    const sides: string[][] = [
      ["backend-test (go unit)", "backend-test"],
      ["backend-test-integration (testcontainers)", "backend-test-integration"],
      ["frontend-test (vitest)", "frontend-test"],
      ["admin-test (vitest)", "admin-test"],
      ["tools-test", "tools-test"],
    ];
    const failures: string[] = [];
    const unknown: string[] = [];
    const passed: string[] = [];
    for (const s of sides) {
      try {
        const r = await world.run("make", ["-C", dir, s[1]], { timeoutMs: SUITE_TIMEOUT_MS });
        if (r.exitCode === 0) passed.push(s[0]);
        else failures.push(s[0] + " exit " + r.exitCode + ": " + (r.stdout + "\n" + r.stderr).slice(-1500));
      } catch (sideErr) {
        unknown.push(s[0] + " — вывод не уместился в лимит воркфлоу");
      }
    }
    return {
      ok: failures.length === 0 && unknown.length === 0,
      how: label + ": make test-log не уместился в лимит вывода, разложено по суитам; зелёные: " + (passed.join(", ") || "нет"),
      failures,
      unknown,
    };
  }
}

/** Правило раскладки файла по области для механического режима (тотальное). */
function deriveRule(p: string): [string, string, string] {
  const seg = p.split("/");
  if (p.startsWith("apps/backend/internal/")) return ["be-" + seg[3], "Бэкенд: " + seg[3], "apps/backend/internal/" + seg[3]];
  if (p.startsWith("apps/backend/db/")) return ["be-db", "Бэкенд: запросы и миграции", "apps/backend/db"];
  if (p.startsWith("apps/backend/")) return ["be-misc", "Бэкенд: прочее", "apps/backend"];
  if (p.startsWith("apps/frontend/widgets/")) return ["fe-w-" + seg[3], "Фронт: widgets/" + seg[3], "apps/frontend/widgets/" + seg[3]];
  if (p.startsWith("apps/frontend/e2e/") || p.startsWith("docs/")) return ["e2e-docs", "E2E и документация", "apps/frontend/e2e"];
  if (p.startsWith("apps/frontend/")) return ["fe-" + seg[2], "Фронт: " + seg[2], "apps/frontend/" + seg[2]];
  if (p.startsWith("apps/admin/")) return ["admin", "Админка", "apps/admin"];
  if (p.startsWith("apps/landing/")) return ["landing", "Лендинг", "apps/landing"];
  return ["other", "Прочее", p];
}

function deriveAreas(files: string[]): AreaDef[] {
  const std = (id: string): string => {
    if (id.startsWith("be-")) return "apps/backend/AGENTS.md, CONTEXT-MAP.md, per-context CONTEXT.md, docs/adr/; деньги — BIGINT копейки, docs/adr/0036.";
    if (id.startsWith("fe-") || id === "e2e-docs") return "apps/frontend/AGENTS.md, apps/frontend/DESIGN.md, docs/testing-strategy.md.";
    if (id === "admin") return "apps/admin/AGENTS.md.";
    return "корневой AGENTS.md, CONTEXT-MAP.md.";
  };
  const groups = new Map<string, string[]>();
  for (const f of files) {
    const id = deriveRule(f)[0];
    const g = groups.get(id);
    if (g) g.push(f);
    else groups.set(id, [f]);
  }
  const areas: AreaDef[] = [];
  for (const [id, fs] of groups) {
    areas.push({
      id,
      title: deriveRule(fs[0])[1],
      paths: Array.from(new Set(fs.map((f) => deriveRule(f)[2]))),
      standards: std(id),
      focus: "Свежим взглядом: дефекты, дубли между тикетами, doc-рот (комментарии про снесённое поведение), каноны области.",
    });
  }
  return areas;
}

async function publishReport(mdText: string, title: string, description: string): Promise<void> {
  try {
    await artifact.markdown("report", mdText, { title, description, primary: true });
  } catch (e) {
    log("Не удалось опубликовать markdown-отчёт: " + String(e).slice(0, 200));
  }
}

// --- Борда для наблюдающего за прогоном ---
artifact.board("verdicts", {
  title: "Находки гейта",
  key: "where",
  status: "status",
  columns: ["Подтверждено", "Правится", "Закоммичено", "Диспозиция", "Неясно"],
  cardTitle: "where",
  detail: [{ field: "what" }, { field: "severity" }, { field: "area" }],
});

// ================================================================
// Фаза 1. Проверка ветки и интеграция свежего dev
// ================================================================
phase("Проверяем ветку и вливаем свежий dev");
const headBranch = await gitOut(WT, ["rev-parse", "--abbrev-ref", "HEAD"]);
if (headBranch !== BRANCH) {
  throw new Error("Ворктри " + WT + " на ветке " + headBranch + ", а конфиг ждёт " + BRANCH);
}
const basePre = await gitOut(WT, ["merge-base", "dev", "HEAD"]);
if (!/^[0-9a-f]{7,40}$/.test(basePre)) {
  throw new Error("merge-base dev/HEAD не разрешился: " + basePre);
}
const devSha = await gitOut(WT, ["rev-parse", "dev"]);
if (devSha === basePre) {
  log("dev уже интегрирован (merge-base = tip dev) — интеграция не нужна");
} else {
  log("dev ушёл вперёд (" + devSha.slice(0, 7) + ") — вливаем в ветку");
  const preMerge = await world.run("git", ["-C", WT, "merge", "--no-ff", "dev"]);
  if (preMerge.exitCode !== 0) {
    log("Интеграция принесла конфликты — подключаю разруливателя");
    const integration = await agent("интегратор dev", PERSONA_INTEGRATOR).ask<FixResult>(
      [
        "В ворктри " + WT + " идёт незавершённый merge dev в ветку " + BRANCH + " — разрули по правилам resolving-merge-conflicts:",
        "1. Увидь состояние: git -C " + WT + " status, список конфликтных файлов.",
        "2. Найди первоисточники каждой стороны: git log обоих родителей конфликтного куска, при необходимости тикеты (gh issue view — читающе).",
        "3. Разрули каждый ханк, сохраняя НАМЕРЕНИЯ ОБОИХ сторон; где несовместимо — выбирай по цели merge (интеграция dev в фичу) и фиксируй развязку в summary. Новое поведение не изобретай, git merge --abort никогда.",
        "4. Известный класс коллизий: номера миграций и ADR, разошедшиеся между веткой и dev — перенумеруй СТОРОНУ ВЕТКИ на следующие свободные номера и поправь все ссылки (включая .squawk.toml, sqlc/openapi-регенераты, CONTEXT-MAP.md).",
        "5. Заверши: git add разрешённых путей и git commit, закрывающий merge. Пуш не делай.",
        "",
        "Верни summary (каждый конфликт и его развязка) и changedFiles.",
      ].join("\n"),
    );
    log("Интеграция завершена: " + integration.summary.slice(0, 200));
  }
  const newBase = await gitOut(WT, ["merge-base", "dev", "HEAD"]);
  const newDev = await gitOut(WT, ["rev-parse", "dev"]);
  if (newBase !== newDev) {
    throw new Error("После интеграции merge-base (" + newBase.slice(0, 7) + ") != tip dev (" + newDev.slice(0, 7) + ") — интеграция не сошлась");
  }
}

// ================================================================
// Фаза 2. Закрепление объёма диффа (уже поверх интегрированного dev)
// ================================================================
phase("Закрепляем объём диффа");
const base = await gitOut(WT, ["merge-base", "dev", "HEAD"]);
const tip = await gitOut(WT, ["rev-parse", "--short", "HEAD"]);
const filesRun = await world.run("git", ["-C", WT, "diff", "--name-only", base + "...HEAD"]);
const allFiles = filesRun.stdout.split("\n").map((s) => s.trim()).filter((s) => s.length > 0);
if (allFiles.length === 0) {
  throw new Error("Дифф ветки против " + base.slice(0, 7) + " пуст — гейт нечего проверять");
}
const commitCount = await gitOut(WT, ["rev-list", "--count", base + "..HEAD"]);
const recentLog = await gitOut(WT, ["log", "--oneline", "-12"]);
log("Гейт закреплён: база " + base.slice(0, 7) + ", кончик " + tip + ", коммитов " + commitCount + ", файлов " + allFiles.length);

const USE_AREAS: AreaDef[] = AREAS.length > 0 ? AREAS.slice() : deriveAreas(allFiles);
const fileToArea = new Map<string, string>();
for (const f of allFiles) {
  if (AREAS.length > 0) {
    let id: string | null = null;
    for (const a of AREAS) {
      for (const q of a.paths) {
        if (f === q || f.startsWith(q + "/")) {
          id = a.id;
          break;
        }
      }
      if (id !== null) break;
    }
    fileToArea.set(f, id ?? "leftovers");
  } else {
    // механический режим: раскладка по тому же правилу, что строит области, — без префикс-скана
    fileToArea.set(f, deriveRule(f)[0]);
  }
}
const uncovered = allFiles.filter((f) => fileToArea.get(f) === "leftovers");
if (uncovered.length > 0) {
  USE_AREAS.push({
    id: "leftovers",
    title: "Файлы вне областей",
    paths: Array.from(new Set(uncovered.map((f) => f.split("/").slice(0, 2).join("/")))),
    standards: "корневой AGENTS.md, CONTEXT-MAP.md.",
    focus: "Файлы, не попавшие в области раскладки: ревьюй по смыслу. Файлы: " + uncovered.join(", "),
  });
  log("Файлов вне областей: " + uncovered.length + " — выделены в отдельную область");
}
const areaFiles = (a: AreaDef): string[] => allFiles.filter((f) => fileToArea.get(f) === a.id);

// ================================================================
// Фаза 3. Полный тест-бейзлайн на ветке
// ================================================================
phase("Гоним полный прогон тестов на ветке");
const baselineRepairer = agent("починильщик бейзлайна", PERSONA_REPAIRER);
let baseline = await runFullSuite(WT, "ветка " + tip);
for (let br = 1; !baseline.ok && baseline.failures.length > 0 && baseline.unknown.length === 0 && br <= BASELINE_REPAIRS; br++) {
  log("Бейзлайн красный, раунд починки " + br + " из " + BASELINE_REPAIRS);
  await baselineRepairer.ask<FixResult>(
    [
      "Ворктри " + WT + " (ветка " + BRANCH + "), полный make test-log красный до начала гейта — почини минимально.",
      "Падение (хвост): " + baseline.failures.join("\n---\n").slice(0, 2500),
      "Полный лог прогона: " + WT + "/.make-test.log — читай из него своё падение целиком.",
      "Целевые тесты для проверки своего фикса: go test / vitest по затронутым файлам; полный сьют снова гонит скрипт после тебя. Не коммить.",
    ].join("\n"),
  );
  baseline = await runFullSuite(WT, "ветка после починки бейзлайна " + br);
}
if (!baseline.ok) {
  const mdBlocked = [
    "# Гейт /pre-merge остановлен: красный тест-бейзлайн",
    "",
    "Ветка `" + BRANCH + "` (" + tip + ", база " + base.slice(0, 7) + "). " + baseline.how,
    "",
    "## Падения",
    "",
    "```",
    ...baseline.failures,
    "```",
    baseline.unknown.length > 0 ? "\n## Не удалось прогнать\n\n" + baseline.unknown.join("; ") + "\n" : "",
    "\nВлитие НЕ сделано. Красный бейзлайн — территория /diagnosing-bugs, не полировки.",
  ].join("\n");
  await publishReport(mdBlocked, "Гейт /pre-merge " + BRANCH + ": красный бейзлайн", baseline.how);
  const blockedBase: GateReportItem[] = baseline.failures.map((f) => ({
    where: WT,
    what: "Тест-бейзлайн красный",
    evidence: f,
    status: "verified" as const,
    severity: "high" as const,
  }));
  return {
    conclusion: "Гейт остановлен до ревью: полный тест-бейзлайн ветки красный (" + baseline.how + "). Влития нет — нужен разбор падений.",
    findings: blockedBase,
    verified: [baseline.how],
    notCovered: [
      ...(baseline.unknown.length > 0 ? ["Суиты, не влезшие в лимит: " + baseline.unknown.join("; ")] : []),
      "ревью и архитектура — до них не дошло",
    ],
    branch: BRANCH,
    merged: false,
    rounds: 0,
  };
}
log("Бейзлайн зелёный: " + baseline.how);

// ================================================================
// Фазы 4–8. Цикл: свип → триаж → фиксы → быстрые гейты → коммиты
// ================================================================
const gateRepairer = agent("починильщик гейтов", PERSONA_REPAIRER);
const committer = agent("коммитёр", PERSONA_COMMITTER);
let converged = false;
let blockedReason = "";
let roundsDone = 0;
let lastBlocking: DisposedFinding[] = [];
const declinesAll: Array<{ label: string; reason: string }> = [];
const issuesFiled: string[] = [];
const appliedClusters: string[] = [];
const summaries: string[] = [];
let unclearHere: DisposedFinding[] = [];
const lowLeftovers: DisposedFinding[] = [];
let dispo = DISPO.slice();

for (let round = 1; round <= SWEEP_ROUNDS; round++) {
  roundsDone = round;
  const roundTag = "R" + round;
  const confirmedHere: DisposedFinding[] = [];
  unclearHere = [];

  phase("Ревьюим ветку и ищем улучшения архитектуры");
  const dispoText = dispo.length > 0 ? dispo.map((s, i) => (i + 1) + ". " + s).join("\n") : "(пока нет — это первый взгляд на ветку)";
  const reviewerAsk = (a: AreaDef): string => {
    const fs = areaFiles(a);
    const filesBlock =
      fs.length > 120
        ? "Файлов в области " + fs.length + " — перечисли их сам: git -C " + WT + " diff --name-only " + base + "...HEAD -- " + a.paths.join(" ") + ". Примеры: " + fs.slice(0, 40).join(", ")
        : "Файлы области (дифф " + base.slice(0, 7) + "...HEAD):\n" + fs.map((p) => "- " + p).join("\n");
    return [
      "Финальный ревью-раунд " + roundTag + " гейта /pre-merge ветки " + BRANCH + " перед влитием в dev.",
      "Читай файлы в ворктри " + WT + " (тот же репозиторий, состояние HEAD " + tip + "). База диффа: " + base.slice(0, 7) + " (merge-base с dev), коммитов ветки: " + commitCount + ".",
      "",
      "ТВОЯ ОБЛАСТЬ: " + a.title + ". " + filesBlock,
      "Стандарты и каноны области: " + a.standards,
      "",
      "ФОКУС: " + a.focus,
      "",
      "Что искать, две оси:",
      "1. ДЕФЕКТЫ и нарушения документированных стандартов (kind=\"defect\"): по AGENTS.md/CONTEXT.md/DESIGN.md/ADR области + smell-бейзлайн Фаулера (Mysterious Name, Duplicated Code, Feature Envy, Data Clumps, Primitive Obsession, Repeated Switches, Shotgun Surgery, Divergent Change, Speculative Generality, Message Chains, Middle Man, Refused Bequest — всегда judgement call, документированный стандарт старше). Обязательно свип doc-рот по всем файлам области: комментарии и докстринги, описывающие поведение, которое ветка удалила или заменила.",
      "2. АРХИТЕКТУРА (kind=\"architecture\"), методика improve-codebase-architecture, сужено на дифф: мелкие модули (интерфейс почти как реализация), дубли форм между тикетами, чистые функции без локальности, пропущенный словарь CONTEXT.md; к подозрительным применяй deletion test. В fixHint — форма углубления. Кандидат за пределами диффа ветки — тоже отмечай: его диспозируют как issue, а не чинят.",
      "",
      "ДИСПОЗИЦИИ ПРОШЛЫХ РАУНДОВ И ПИНЫ (это НЕ находки, не переоткрывай):",
      dispoText,
      "",
      "Гейты уже зелёные (полный make test, tsc/eslint/vitest/go) — сьюты не гоняй. Ты read-only: файлы не редактируй, make/docker/e2e не запускай, git — только читающие команды. Дифф смотреть: git -C " + WT + " diff " + base + "...HEAD -- <путь>.",
      "",
      "Серьёзность: high — потеря данных/крэш/реальная регрессия; medium — нарушение канона/контракта или сильный арх-кандидат; low — мелочь. Каждую находку подкрепляй путь:строка и цитатой. Верни summary и findings.",
    ].join("\n");
  };
  const confirmAsk = (f: RawFinding): string =>
    [
      "Независимо проверь находку ревьюера гейта ветки " + BRANCH + " (ворктри " + WT + ", состояние HEAD " + tip + ").",
      "Находка (JSON): " + JSON.stringify(f),
      "",
      "Воспроизведи с нуля: прочитай указанное место и окружение, при необходимости смежный код и читающие git-команды (git -C " + WT + " ...). Не доверяй формулировке автора: возможно, он misread контекст, канал или диспозицию. Не редактируй файлы, сьюты не запускай.",
      f.kind === "architecture"
        ? "Для арх-кандидата подтверди: (а) трение реально, (б) предлагаемый углубляющий фикс сохраняет поведение, (в) он лежит в диффе ветки. Если фикс требует смены поведения — верди «Подтверждено», но напиши об этом в reason."
        : "",
      "Верни вердикт «Подтверждено» (реально и серьёзность адекватна), «Опровергнуто» (находки нет — объясни со ссылками) или «Неясно» и обоснование путь:строка.",
    ]
      .filter((s) => s.length > 0)
      .join("\n");

  const specTask = async (): Promise<AreaResult> => {
    const review = await agent("ревьюер спеки " + roundTag, PERSONA_SPEC_REVIEWER).ask<ReviewOutcome>(
      [
        "Проверь дифф ветки " + BRANCH + " (ворктри " + WT + ", база " + base.slice(0, 7) + "...HEAD, кончик " + tip + ") на соответствие спеке.",
        "Спека: карта " + MAP_ISSUE + " — прочитай её и связанные тикеты читающими командами gh (issue view, комментарии).",
        "Ищи (kind=\"spec\"): (а) требования спеки, которых нет или они частичные; (б) поведение в диффе, которого спека не просила (scope creep); (в) реализованное, но выглядящее неправильно. Каждую находку подкрепляй цитатой спеки и путь:строка. Пустой список — честный ответ. Файлы не редактируй, сьюты не гоняй.",
      ].join("\n"),
    );
    const judged: JudgedFinding[] = await Promise.all(
      review.findings.map(async (f, i): Promise<JudgedFinding> => {
        const conf = await agent("проверка спеки " + roundTag + " · " + (i + 1), PERSONA_CONFIRMER).ask<Confirmation>(confirmAsk(f));
        return { ...f, area: "Спека " + MAP_ISSUE, verdict: conf.verdict, confirmReason: conf.reason };
      }),
    );
    return { title: "Спека " + MAP_ISSUE, summary: review.summary, findings: judged };
  };

  const laneResults: AreaResult[] = await Promise.all([
    ...USE_AREAS.map(async (a): Promise<AreaResult> => {
      const review = await agent("ревьюер " + roundTag + " · " + a.title, PERSONA_REVIEWER).ask<ReviewOutcome>(reviewerAsk(a));
      log(a.title + ": " + review.findings.length + " находок");
      const judged: JudgedFinding[] = await Promise.all(
        review.findings.map(async (f, i): Promise<JudgedFinding> => {
          const conf = await agent("проверка " + roundTag + " · " + a.id + " · " + (i + 1), PERSONA_CONFIRMER).ask<Confirmation>(confirmAsk(f));
          return { ...f, area: a.title, verdict: conf.verdict, confirmReason: conf.reason };
        }),
      );
      return { title: a.title, summary: review.summary, findings: judged };
    }),
    ...(MAP_ISSUE ? [specTask()] : []),
  ]);

  let fid = 0;
  let refutedCount = 0;
  for (const r of laneResults) {
    summaries.push(roundTag + " · " + r.title + ": " + r.summary);
    for (const f of r.findings) {
      const df: DisposedFinding = { ...f, id: fid++, round };
      if (f.verdict === "Подтверждено") {
        confirmedHere.push(df);
        report({ where: df.where, status: "Подтверждено", what: df.what, severity: df.severity, area: df.area, round }, "verdicts");
      } else if (f.verdict === "Опровергнуто") {
        refutedCount++;
        declinesAll.push({ label: roundTag + " опровергнуто " + df.where, reason: df.confirmReason });
        report({ where: df.where, status: "Диспозиция", what: "Опровергнуто: " + df.what, severity: df.severity, area: df.area, round }, "verdicts");
      } else {
        unclearHere.push(df);
        report({ where: df.where, status: "Неясно", what: df.what, severity: df.severity, area: df.area, round }, "verdicts");
      }
    }
  }
  // После раунда 2 оставшиеся low не блокируют: уходят в осознанный остаток (в отчёт, без правок).
  const lowsBlock = round <= 2;
  const blockingConfirmed = lowsBlock ? confirmedHere : confirmedHere.filter((f) => f.severity !== "low");
  const blockingUnclear = lowsBlock ? unclearHere : unclearHere.filter((f) => f.severity !== "low");
  if (blockingConfirmed.length > 0) {
    lastBlocking = blockingConfirmed;
  }
  if (!lowsBlock) {
    for (const f of confirmedHere) {
      if (f.severity === "low") lowLeftovers.push(f);
    }
  }
  log("Раунд " + roundTag + ": подтверждено " + confirmedHere.length + " (блокирующих " + blockingConfirmed.length + "), опровергнуто " + refutedCount + ", неясных " + unclearHere.length);

  if (blockingConfirmed.length === 0 && blockingUnclear.length === 0) {
    converged = true;
    if (lowLeftovers.length > 0) {
      log("Low-находки (" + lowLeftovers.length + ") уходят в осознанный остаток без правок");
      dispo.push("Осознанный остаток: low-находки приняты без правок — " + lowLeftovers.map((f) => f.where).join("; "));
    }
    log("Раунд " + roundTag + " чистый — гейт сошёлся");
    break;
  }
  if (round === SWEEP_ROUNDS) {
    blockedReason = "потолок " + SWEEP_ROUNDS + " ревью-раундов: остаются блокирующие находки (" + blockingConfirmed.length + ")";
    break;
  }

  phase("Раскладываем находки по кластерам правок");
  const triage = await agent("триажёр " + roundTag, PERSONA_TRIAGE).ask<TriageOutcome>(
    [
      "Раскладываешь подтверждённые находки раунда " + roundTag + " гейта ветки " + BRANCH + " (ворктри " + WT + ") по кластерам правок.",
      "Правила кластеров: НЕ пересекаются по файлам (фиксятся параллельно); кластер = один будущий коммит; в task — конкретные инструкции по каждой находке (путь:строка, что сделать; behavior-смежное — test-first: падающий тест → фикс → зелёный); commitMsg — одна строка в каноне репозитория; правки сохраняют поведение; сгенерированное (generated.ts, openapi, sqlc) в task не трогается.",
      "Арх-кандидаты: Speculative — в declines с причиной; Strong/Worth exploring, лежащие в диффе и сохраняющие поведение — в кластеры; фикс требует смены поведения — СРАЗУ задай владельцу вопрос (эскалация): что менять, варианты, рекомендация — и действуй по ответу; кандидат за пределами бласт-радиуса ветки — в issuesToFile (title/body по-русски, с доказательствами путь:строка).",
      "Неясные находки: либо разберись сам по коду и включи в кластер, либо задай владельцу вопрос «принять как пин?» и запиши развязку в note.",
      "Каждая находка обязана получить исход: кластер, decline или issue. Неиспользованных id не оставляй.",
      "",
      "ПОДТВЕРЖДЁННЫЕ (JSON, id — твой ключ): " + JSON.stringify(confirmedHere),
      "",
      "НЕЯСНЫЕ (JSON): " + JSON.stringify(unclearHere),
      "",
      "Стиль сообщений коммитов (последние коммиты ветки):",
      recentLog,
    ].join("\n"),
  );

  let clusters = triage.clusters.slice();
  if (clusters.length > 1) {
    let mergedAny = true;
    while (mergedAny) {
      mergedAny = false;
      outer: for (let i = 0; i < clusters.length; i++) {
        for (let j = i + 1; j < clusters.length; j++) {
          if (clusters[j].files.some((f) => clusters[i].files.includes(f))) {
            const a = clusters[i];
            const b = clusters[j];
            clusters[i] = {
              id: a.id,
              title: a.title + " + " + b.title,
              order: Math.min(a.order, b.order),
              files: Array.from(new Set([...a.files, ...b.files])),
              task: a.task + "\n\n=== Второй кластер того же коммита ===\n" + b.task,
              commitMsg: a.commitMsg + "; " + b.commitMsg,
              findingIds: [...a.findingIds, ...b.findingIds],
            };
            clusters.splice(j, 1);
            mergedAny = true;
            break outer;
          }
        }
      }
    }
  }
  const claimed = new Set<number>();
  for (const c of clusters) for (const fidT of c.findingIds) claimed.add(fidT);
  for (const d of triage.declines) claimed.add(d.findingId);
  for (const it of triage.issuesToFile) claimed.add(it.findingId);
  const missing = confirmedHere.filter((f) => !claimed.has(f.id));
  if (missing.length > 0) {
    log("Триаж не распределил находок: " + missing.length + " — добавляю резервный кластер");
    clusters.push({
      id: "reserve",
      title: "Резервный кластер",
      order: 99,
      files: Array.from(new Set(missing.map((f) => f.where.split(":")[0]))),
      task: missing.map((f) => "- " + f.where + ": " + f.what + " (" + (f.fixHint || "фикс по смыслу") + ")").join("\n"),
      commitMsg: "fix: гейт " + BRANCH + " — резервный кластер находок раунда " + roundTag,
      findingIds: missing.map((f) => f.id),
    });
  }
  clusters.sort((a, b) => a.order - b.order);
  for (const d of triage.declines) {
    const f = confirmedHere.find((x) => x.id === d.findingId);
    declinesAll.push({ label: f ? f.where : String(d.findingId), reason: d.reason });
    if (f) report({ where: f.where, status: "Диспозиция", what: "Отклонено: " + d.reason.slice(0, 80), severity: f.severity, area: f.area, round }, "verdicts");
  }
  for (const it of triage.issuesToFile) {
    const created = await world.run("gh", ["issue", "create", "--title", it.title, "--body", it.body]);
    const lastLine = created.stdout.trim().split("\n").pop() || "";
    const url = created.exitCode === 0 && lastLine.startsWith("http") ? lastLine : "ОШИБКА: " + (created.stderr || created.stdout).slice(0, 200);
    issuesFiled.push(it.title + " — " + url);
    log("Зафайлован issue: " + it.title + " → " + url);
  }
  if (triage.note.length > 0) {
    dispo.push("Раунд " + roundTag + ", развязки триажа: " + triage.note);
  }
  if (clusters.length === 0) {
    converged = true;
    dispo.push("Раунд " + roundTag + ": находки диспозированы без правок — " + triage.note);
    log("Все находки раунда диспозированы без правок");
    break;
  }
  for (const c of clusters) {
    for (const fidT of c.findingIds) {
      const f = confirmedHere.find((x) => x.id === fidT);
      if (f) report({ where: f.where, status: "Правится", what: f.what, severity: f.severity, area: f.area, round }, "verdicts");
    }
  }

  phase("Чиним кластеры находок");
  const fixResults = await Promise.all(
    clusters.map(async (c): Promise<{ cluster: Cluster; result: FixResult }> => {
      const result = await agent("фиксер " + roundTag + " · " + c.title, PERSONA_FIXER).ask<FixResult>(
        [
          "Контекст: ворктри " + WT + " (ветка " + BRANCH + ", HEAD " + tip + "). Владелец постановил исправлять все находки гейта до нуля.",
          "Целевые тесты: фронт — npx vitest run <пути> из " + WT + "/apps/frontend; бэк — go test ./... из " + WT + "/apps/backend (или go test <пакет>).",
          "",
          "ТВОЙ КЛАСТЕР: " + c.title + ".",
          "",
          c.task,
        ].join("\n"),
      );
      log(c.title + ": правки готовы (" + result.changedFiles.length + " файлов)");
      return { cluster: c, result };
    }),
  );

  phase("Гоним быстрые проверки и чиним падения");
  const runTsc = () => world.run("node", [WT + "/apps/frontend/node_modules/typescript/bin/tsc", "--noEmit", "-p", WT + "/apps/frontend"], { timeoutMs: 300_000 });
  const runLint = () => world.run("npm", ["--prefix", WT + "/apps/frontend", "run", "lint"], { timeoutMs: 600_000 });
  const runVitest = () => world.run("make", ["-C", WT, "frontend-test"], { timeoutMs: GATE_TIMEOUT_MS });
  const runGo = () => world.run("make", ["-C", WT, "backend-test"], { timeoutMs: GATE_TIMEOUT_MS });
  const runBackendLint = () => world.run("make", ["-C", WT, "backend-lint"], { timeoutMs: GATE_TIMEOUT_MS });
  let gateFailures: GateFailure[] = [];
  for (let gr = 1; gr <= REPAIR_ROUNDS; gr++) {
    gateFailures = [];
    const tsc = await runTsc();
    if (tsc.exitCode !== 0) gateFailures.push({ gate: "tsc", output: (tsc.stdout + "\n" + tsc.stderr).slice(-2000) });
    const lint = await runLint();
    if (lint.exitCode !== 0) gateFailures.push({ gate: "eslint", output: (lint.stdout + "\n" + lint.stderr).slice(-2000) });
    const vitest = await runVitest();
    if (vitest.exitCode !== 0) gateFailures.push({ gate: "vitest (make frontend-test)", output: (vitest.stdout + "\n" + vitest.stderr).slice(-2500) });
    const go = await runGo();
    if (go.exitCode !== 0) gateFailures.push({ gate: "go test (make backend-test)", output: (go.stdout + "\n" + go.stderr).slice(-2500) });
    const goLint = await runBackendLint();
    if (goLint.exitCode !== 0) gateFailures.push({ gate: "backend-lint (make backend-lint)", output: (goLint.stdout + "\n" + goLint.stderr).slice(-2500) });
    log("Быстрые гейты (попытка " + gr + "): " + (gateFailures.length === 0 ? "все зелёные" : "красные: " + gateFailures.map((f) => f.gate).join(", ")));
    if (gateFailures.length === 0) break;
    if (gr === REPAIR_ROUNDS) break;
    await gateRepairer.ask<FixResult>(
      [
        "Ворктри " + WT + " (ветка " + BRANCH + "): быстрые гейты красные после правок кластеров. Почини минимально, не меняя смысл диспозиций. Падения (JSON): " + JSON.stringify(gateFailures),
        "Полный сьют после тебя снова гонит скрипт; e2e/docker не трогай, не коммить.",
      ].join("\n"),
    );
  }
  if (gateFailures.length > 0) {
    blockedReason = "быстрые гейты красные после " + REPAIR_ROUNDS + " раундов починки — по правилу гейта коммиты не делаются";
    break;
  }

  phase("Коммитим кластеры по одному");
  const appliedThisRound: string[] = [];
  for (const fr of fixResults) {
    const files = Array.from(new Set([...fr.cluster.files, ...fr.result.changedFiles]));
    const commit = await committer.ask<CommitResult>(
      [
        "Ворктри: " + WT + ". Сделай ОДИН коммит: git add -- <перечисленные пути>, затем git commit с сообщением из кавычек ниже (сообщение не менять). Если какой-то путь не существует — пропусти его; если добавлять нечего — верни hash «нет изменений» и не создавай пустой коммит. Пуш не делай.",
        "Пути (добавляй только их): " + JSON.stringify(files),
        "Сообщение: " + fr.cluster.commitMsg,
      ].join("\n"),
    );
    log(fr.cluster.title + " → коммит " + commit.hash);
    appliedThisRound.push(fr.cluster.title + " (" + commit.hash + ")");
    appliedClusters.push(fr.cluster.title + " (" + commit.hash + ")");
    for (const fidT of fr.cluster.findingIds) {
      const f = confirmedHere.find((x) => x.id === fidT);
      if (f) report({ where: f.where, status: "Закоммичено", what: commit.hash + " " + commit.subject.slice(0, 70), severity: f.severity, area: f.area, round }, "verdicts");
    }
  }
  const wtStatusMid = await world.run("git", ["-C", WT, "status", "--porcelain"]);
  const dirtyMid = wtStatusMid.stdout.split("\n").map((s) => s.trim()).filter((s) => s.length > 0);
  if (dirtyMid.length > 0) {
    log("После коммитов в ворктри остались незакоммиченные правки: " + dirtyMid.length + " записей");
    dispo.push("Раунд " + roundTag + ": после кластеров остались незакоммиченные правки — " + dirtyMid.slice(0, 10).join("; "));
  } else {
    dispo.push("Раунд " + roundTag + " исправлен и закоммичен (" + appliedThisRound.join("; ") + ") — не переоткрывать, искать новое.");
  }
}

// ================================================================
// Блокировка: потолок раундов или красные гейты — влития нет
// ================================================================
if (blockedReason.length > 0) {
  const wtStatus = await world.run("git", ["-C", WT, "status", "--porcelain"]);
  const dirty = wtStatus.stdout.split("\n").map((s) => s.trim()).filter((s) => s.length > 0);
  const mdBlocked = [
    "# Гейт /pre-merge остановлен: " + blockedReason,
    "",
    "Ветка `" + BRANCH + "` (" + tip + ", база " + base.slice(0, 7) + "), раундов сделано: " + roundsDone + ".",
    "",
    "## Сводки областей по раундам",
    "",
    ...summaries.map((s) => "- " + s),
    "",
    lastBlocking.length > 0 ? "## Блокирующие находки последнего раунда (поимённо)\n\n" + lastBlocking.map((f) => "- `" + f.where + "` — " + f.what + " _(" + f.severity + ", " + f.kind + "; " + f.confirmReason + ")_").join("\n") + "\n" : "",
    declinesAll.length > 0 ? "## Диспозиции\n\n" + declinesAll.map((d) => "- " + d.label + " — " + d.reason).join("\n") + "\n" : "",
    unclearHere.length > 0 ? "## Неясные находки (не подтверждены и не опровергнуты)\n\n" + unclearHere.map((f) => "- `" + f.where + "` — " + f.what + " _(" + f.confirmReason + ")_").join("\n") + "\n" : "",
    lowLeftovers.length > 0 ? "## Low-остаток (после раунда 2 не блокирует, принято без правок)\n\n" + lowLeftovers.map((f) => "- `" + f.where + "` — " + f.what).join("\n") + "\n" : "",
    issuesFiled.length > 0 ? "## Зафайлованные issues\n\n" + issuesFiled.map((s) => "- " + s).join("\n") + "\n" : "",
    dirty.length > 0 ? "## Незакоммиченный остаток в ворктри (при красных гейтах коммиты не делаются)\n\n```\n" + dirty.join("\n") + "\n```\n" : "",
    "\nВлитие НЕ сделано: осознанный остаток уходит владельцу.",
  ]
    .filter((s) => s.length > 0)
    .join("\n");
  await publishReport(mdBlocked, "Гейт /pre-merge " + BRANCH + ": " + blockedReason, "Раундов: " + roundsDone + "; влитие не сделано");
  return {
    conclusion: "Гейт остановлен: " + blockedReason + ". Влития нет — остаток уходит владельцу как осознанный.",
    findings: [
      ...unclearHere.map((f) => ({ where: f.where, what: f.what, evidence: f.confirmReason, status: "unconfirmed" as const, severity: f.severity })),
      ...lowLeftovers.map((f) => ({ where: f.where, what: f.what, evidence: f.evidence, status: "verified" as const, severity: "low" as const })),
    ],
    verified: [
      baseline.how,
      "Ревью-раундов: " + roundsDone + " по " + USE_AREAS.length + " областям, каждая находка проверена независимо",
      "Диспозиций: " + declinesAll.length + ", зафайловано issues: " + issuesFiled.length,
    ],
    notCovered: ["влитие в dev — заблокировано", "финальный полный прогон — не понадобился"],
    branch: BRANCH,
    merged: false,
    rounds: roundsDone,
  };
}

// ================================================================
// Фаза 9. Финальный полный прогон
// ================================================================
phase("Гоним финальный полный прогон тестов");
const tipFinal = await gitOut(WT, ["rev-parse", "--short", "HEAD"]);
const finalSuite = await runFullSuite(WT, "итоговое дерево ветки " + tipFinal);
if (!finalSuite.ok) {
  const mdBlocked = [
    "# Гейт /pre-merge остановлен: финальный полный прогон красный",
    "",
    "Ветка `" + BRANCH + "` (" + tipFinal + "). Ревью сошлось за " + roundsDone + " раунд(а/ов), но финальный полный прогон не зелёный. " + finalSuite.how,
    "",
    "```",
    ...finalSuite.failures,
    "```",
    finalSuite.unknown.length > 0 ? "\nНе удалось прогнать: " + finalSuite.unknown.join("; ") + "\n" : "",
    "\nВлитие НЕ сделано.",
  ].join("\n");
  await publishReport(mdBlocked, "Гейт /pre-merge " + BRANCH + ": финальный прогон красный", finalSuite.how);
  return {
    conclusion: "Ревью сошлось, но финальный полный прогон на " + tipFinal + " красный — влития нет.",
    findings: finalSuite.failures.map((f) => ({ where: WT, what: "Финальный полный прогон красный", evidence: f, status: "verified" as const, severity: "high" as const })),
    verified: ["Ревью-раундов: " + roundsDone, finalSuite.how],
    notCovered: ["влитие в dev — заблокировано"],
    branch: BRANCH,
    merged: false,
    rounds: roundsDone,
  };
}

// ================================================================
// Фаза 10. Влитие в dev (главный чекаут)
// ================================================================
phase("Проверяем готовность dev к влитию");
const wtStatusFinal = await world.run("git", ["-C", WT, "status", "--porcelain"]);
const dirtyFinal = wtStatusFinal.stdout.split("\n").map((s) => s.trim()).filter((s) => s.length > 0);
if (dirtyFinal.length > 0) {
  throw new Error("Ворктри не чист перед влитием: " + dirtyFinal.slice(0, 10).join("; "));
}
const mainBranch = await gitOut(MAIN, ["rev-parse", "--abbrev-ref", "HEAD"]);
if (mainBranch !== "dev") {
  throw new Error("Главный чекаут " + MAIN + " на ветке " + mainBranch + " — влитие идёт только из dev");
}
const devInMain = await gitOut(MAIN, ["rev-parse", "dev"]);
const devInWt = await gitOut(WT, ["rev-parse", "dev"]);
if (devInMain !== devInWt) {
  throw new Error("dev сдвинулся во время гейта (в чекауте " + devInMain.slice(0, 7) + ", ветка интегрировала " + devInWt.slice(0, 7) + ") — нужен повторный прогон после интеграции");
}
const mergeTree = await world.run("git", ["-C", MAIN, "merge-tree", "--write-tree", "dev", BRANCH]);
if (mergeTree.exitCode !== 0) {
  throw new Error("merge-tree предсказал конфликт при влитии " + BRANCH + " в dev — dev ушёл вперёд во время гейта? Нужен повторный прогон: " + (mergeTree.stdout || mergeTree.stderr).slice(0, 400));
}
const mergesLog = await gitOut(MAIN, ["log", "--merges", "--oneline", "-5"]);

if (ASK_BEFORE_MERGE) {
  phase("Финальный вопрос перед влитием");
  const decisionRaw = await agent("страж влития", PERSONA_GUARD).ask<string>(
    [
      "Гейт ветки " + BRANCH + " сошёлся полностью: блокирующих находок нет, финальный полный прогон зелёный (" + finalSuite.how + ").",
      "Задай владельцу ОДИН финальный вопрос (эскалация): вливать ветку в dev сейчас? Контекст: карта " + (MAP_ISSUE || "не указана") + ", ревью-раундов " + roundsDone + ", коммитов ветки " + commitCount + ", дерево ворктри чистое.",
      "Верни ровно merge, если владелец подтвердил; hold и причину одной строкой — если отложил.",
    ].join("\n"),
  );
  const decision = decisionRaw.trim().toLowerCase();
  if (!decision.startsWith("merge")) {
    const holdReason = decisionRaw.trim().replace(/^hold[:,\s]*/i, "").trim() || "владелец отложил влитие";
    const mdHold = [
      "# Гейт /pre-merge `" + BRANCH + "` — пройден, влитие отложено владельцем",
      "",
      "Все проверки зелёные (" + finalSuite.how + "), блокирующих находок нет. Влитие отложено: " + holdReason,
      "",
      "Ветка `" + tipFinal + "` готова к влитию, когда будет слово: git -C " + MAIN + " merge --no-ff " + BRANCH,
    ].join("\n");
    await publishReport(mdHold, "Гейт /pre-merge " + BRANCH + ": влитие отложено", holdReason);
    return {
      conclusion: "Гейт пройден полностью (0 блокирующих находок, прогон зелёный), влитие отложено решением владельца: " + holdReason,
      findings: lowLeftovers.map((f) => ({ where: f.where, what: f.what, evidence: f.evidence, status: "verified" as const, severity: "low" as const })),
      verified: [baseline.how, finalSuite.how, "Ревью-раундов: " + roundsDone + ", каждая находка проверена независимо"],
      notCovered: ["влитие в dev — отложено владельцем", "e2e — не гонялся (гоняется после влития, если включён E2E_AFTER_MERGE)"],
      branch: BRANCH,
      merged: false,
      rounds: roundsDone,
    };
  }
  log("Владелец подтвердил влитие");
}

phase("Составляем сообщение и вливаем ветку");
const mergeMsg = await agent("редактор слияния", PERSONA_EDITOR).ask<string>(
  [
    "Составь строку сообщения для merge ветки " + BRANCH + " в dev.",
    "Контекст усилия: " + (MAP_ISSUE ? "карта " + MAP_ISSUE : "карта не указана — собери из коммитов") + ".",
    "Коммиты ветки:",
    recentLog,
    "Кластеры гейта: " + (appliedClusters.join("; ") || "правок гейт не вносил"),
    "Раундов ревью: " + roundsDone + ", финальный полный прогон: " + finalSuite.how,
    "Канон репозитория (последние merge-коммиты):",
    mergesLog,
  ].join("\n"),
);
const mergeRun = await world.run("git", ["-C", MAIN, "merge", "--no-ff", BRANCH, "-m", mergeMsg]);
if (mergeRun.exitCode !== 0) {
  throw new Error("git merge не прошёл: " + (mergeRun.stdout + "\n" + mergeRun.stderr).slice(0, 600));
}
const mergeHash = await gitOut(MAIN, ["rev-parse", "--short", "HEAD"]);
log("Влито: merge " + mergeHash + " в dev");

// ================================================================
// Фаза 11. Полный прогон на слитом dev
// ================================================================
phase("Проверяем слитый dev тестами");
const devSuite = await runFullSuite(MAIN, "слитый dev (" + mergeHash + ")");
const mergeFindings: GateReportItem[] = [];
if (!devSuite.ok) {
  for (const f of devSuite.failures) {
    mergeFindings.push({
      where: MAIN,
      what: "Полный прогон на слитом dev красный — merge " + mergeHash + " требует живого разбора",
      evidence: f,
      status: "verified",
      severity: "high",
    });
  }
}

let e2eNote = "не гонялся (E2E_AFTER_MERGE выключен)";
if (E2E_AFTER_MERGE) {
  phase("Гоним e2e на слитом dev");
  try {
    const e2e = await world.run("make", ["-C", MAIN, "frontend-e2e"], { timeoutMs: 3_600_000 });
    if (e2e.exitCode === 0) {
      e2eNote = "make frontend-e2e на слитом dev exit 0";
      log("e2e на слитом dev зелёный");
    } else {
      e2eNote = "make frontend-e2e на слитом dev КРАСНЫЙ (exit " + e2e.exitCode + ")";
      mergeFindings.push({
        where: MAIN,
        what: "e2e на слитом dev красный — нужен живой разбор (merge уже сделан)",
        evidence: (e2e.stdout + "\n" + e2e.stderr).slice(-2000),
        status: "verified",
        severity: "high",
      });
    }
  } catch (e2eCap) {
    e2eNote = "вывод e2e не уместился в лимит воркфлоу — прогнать живьём";
    mergeFindings.push({
      where: MAIN,
      what: "e2e на слитом dev не прогнан воркфлоу (лимит вывода) — прогнать живьём",
      evidence: String(e2eCap).slice(0, 200),
      status: "unconfirmed",
      severity: "medium",
    });
  }
}

// ================================================================
// Фаза 12. Отчёт в карту и публикация
// ================================================================
const md = [
  "# Гейт /pre-merge ветки `" + BRANCH + "` — пройден, влит в dev",
  "",
  "Ветка `" + tipFinal + "` (база " + base.slice(0, 7) + ", " + commitCount + " коммитов, " + allFiles.length + " файлов). Ревью-раундов: " + roundsDone + ". Влитие: merge " + mergeHash + " (--no-ff). Пуш НЕ делается — по слову владельца.",
  "",
  "## Ход гейта",
  "",
  "- Полный прогон на ветке: " + baseline.how,
  "- Цикл полировки: " + roundsDone + " ревью-раунд(а/ов); кластеры: " + (appliedClusters.join("; ") || "правок не потребовалось"),
  lowLeftovers.length > 0 ? "- Low-остаток (принято без правок, после раунда 2 не блокирует): " + lowLeftovers.map((f) => f.where).join("; ") : "",
  "- Финальный полный прогон: " + finalSuite.how,
  "- Полный прогон на слитом dev: " + devSuite.how,
  "- e2e на слитом dev: " + e2eNote,
  "",
  "## Сводки областей по раундам",
  "",
  ...summaries.map((s) => "- " + s),
  "",
  declinesAll.length > 0 ? "## Диспозиции (отклонено с причинами)\n\n" + declinesAll.map((d) => "- " + d.label + " — " + d.reason).join("\n") + "\n" : "",
  issuesFiled.length > 0 ? "## Зафайлованные issues\n\n" + issuesFiled.map((s) => "- " + s).join("\n") + "\n" : "",
  mergeFindings.length > 0 ? "## ⚠️ Слитый dev красный\n\nЖивой разбор: " + devSuite.failures.join(" | ").slice(0, 800) + "\n" : "",
  "## Хвосты (вне гейта, по слову владельца)",
  "",
  "- Пуш в origin — только по слову владельца (пуш dev = деплой stage).",
  "- Разборка слота/ворктри по docs/agents/parallel-dev.md (стенды гасить до worktree remove).",
  "- Для UI-веток: живой ui-walkthrough остаётся отдельной приёмкой.",
]
  .filter((s) => s.length > 0)
  .join("\n");

if (MAP_ISSUE) {
  phase("Публикуем отчёт в карту");
  const reporter = agent("репортёр", PERSONA_REPORTER);
  const posted = await reporter.ask<string>(
    [
      "Размести отчёт ниже комментарием в issue " + MAP_ISSUE + " (gh issue comment, тело — текст между маркерами БЕЗ самих маркеров), затем закрой issue (gh issue close).",
      "---НАЧАЛО-ОТЧЁТА---",
      md,
      "---КОНЕЦ-ОТЧЁТА---",
    ].join("\n"),
  );
  log("Отчёт опубликован: " + posted.slice(0, 160));
}

await publishReport(md, "Гейт /pre-merge " + BRANCH, "Влит в dev: merge " + mergeHash + "; раундов ревью: " + roundsDone);

return {
  conclusion:
    "Гейт " + BRANCH + " пройден за " + roundsDone + " ревью-раунд(а/ов): все блокирующие находки исправлены и закоммичены" +
    (lowLeftovers.length > 0 ? " (low-остаток: " + lowLeftovers.length + " шт., принят без правок)" : "") +
    ", полные прогоны зелёные" +
    (devSuite.ok ? " и на слитом dev" : ", НО слитый dev красный — нужен живой разбор") +
    (!E2E_AFTER_MERGE ? "" : e2eNote.includes("exit 0") ? ", e2e на dev зелёный" : ", e2e на dev требует разбора") +
    ". Влито: merge " + mergeHash + " (--no-ff). Пуш не делается — по слову владельца.",
  findings: mergeFindings,
  verified: [
    baseline.how,
    finalSuite.how,
    devSuite.how,
    ...(E2E_AFTER_MERGE ? [e2eNote] : []),
    "Ревью: " + roundsDone + " раунд(а/ов) по " + USE_AREAS.length + " областям, каждая находка подтверждена независимо",
    "Быстрые гейты перед каждым коммитом: tsc, eslint, vitest, go test",
    "Кластеров правок закоммичено: " + appliedClusters.length,
    ...(lowLeftovers.length > 0 ? ["Low-остаток принят без правок: " + lowLeftovers.length + " находок (в отчёте)"] : []),
  ],
  notCovered: [
    "e2e-сьют в ворктри — вне гейта (грабля слотов/.env)" + (E2E_AFTER_MERGE ? "" : "; после-мердж e2e выключен (E2E_AFTER_MERGE=false)"),
    ...(devSuite.unknown.length > 0 ? ["Суиты, не влезшие в лимит на слитом dev: " + devSuite.unknown.join("; ")] : []),
    "пуш и разборка ворктри/стендов — по слову владельца",
  ],
  branch: BRANCH,
  merged: true,
  mergeCommit: mergeHash,
  rounds: roundsDone,
};
