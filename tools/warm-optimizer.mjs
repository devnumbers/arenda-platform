#!/usr/bin/env node
// Локальный прогрев кэша /_next/image лендинга после деплоя — ручной аналог
// вырезанного шага Warm image optimizer из .github/workflows/_deploy.yml
// (вырезан 07.10.2026 по решению владельца: на деградировавшем хосте энкоды
// не влезали в таймаут Caddy и валили деплой с автооткатом; здесь падение
// одного URL ничего не откатывает — просто отчёт).
//
// Usage:
//   node tools/warm-optimizer.mjs [URL] [--concurrency N] [--limit N] [--retries N] [--all]
//
//   URL              цель, по умолчанию https://dev.rentlee.ru (прод: https://rentlee.ru)
//   --concurrency N  параллельных запросов (умолчание 3; больше — сильнее душит контейнер)
//   --limit N        прогреть только первые N URL (проверка скрипта)
//   --retries N      попыток на URL (умолчание 4, как curl --retry 3 в воркфлоу)
//   --all            без фильтра url=%2Flanding%2F — греть и кабинетный оптимизатор
//
// Exit: 0 — всё прогрелось, 1 — часть URL не ответила (список в конце).

const argv = process.argv.slice(2);
const flag = (name, fallback) => {
  const i = argv.indexOf(name);
  return i === -1 ? fallback : Number(argv[i + 1]);
};
const has = (name) => argv.includes(name);
const positional = argv.find((a) => !a.startsWith("--") && isNaN(Number(a)));

const BASE_URL = (positional ?? "https://dev.rentlee.ru").replace(/\/+$/, "");
const CONCURRENCY = flag("--concurrency", 3);
const LIMIT = flag("--limit", 0);
const RETRIES = flag("--retries", 4);
const TIMEOUT_MS = 90_000; // как --max-time 90 в воркфлоу
const RETRY_DELAY_MS = 4_000;
const ACCEPT = "image/avif,image/webp,image/*,*/*;q=0.8";

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function fetchWithRetry(url, { attempts, accept }) {
  for (let i = 1; i <= attempts; i++) {
    try {
      const res = await fetch(url, {
        headers: accept ? { Accept: ACCEPT } : {},
        signal: AbortSignal.timeout(TIMEOUT_MS),
      });
      if (res.ok) return res;
      console.error(`  попытка ${i}/${attempts}: HTTP ${res.status}`);
    } catch (e) {
      console.error(`  попытка ${i}/${attempts}: ${e.cause?.code ?? e.name}`);
    }
    if (i < attempts) await sleep(RETRY_DELAY_MS);
  }
  return null;
}

// HTML главной → уникальные URL оптимизатора (та же выборка, что была в шаге).
const htmlRes = await fetchWithRetry(`${BASE_URL}/`, { attempts: 5 });
if (!htmlRes) {
  console.error(`Главная ${BASE_URL}/ не отвечает — прогревать нечего.`);
  process.exit(1);
}
const html = await htmlRes.text();
let urls = [...html.matchAll(/\/_next\/image\?[^" ]+/g)]
  .map((m) => m[0])
  // &amp; из HTML-атрибутов и \u0026 из RSC-пейлоада разворачиваем назад в & —
  // иначе урл склеивается в мусор вида «webpu0026w=96».
  .map((u) => u.replaceAll("&amp;", "&").replaceAll("\\u0026", "&"))
  .filter((u) => has("--all") || u.includes("url=%2Flanding%2F"));
urls = [...new Set(urls)];
if (LIMIT > 0) urls = urls.slice(0, LIMIT);
if (urls.length === 0) {
  console.error("В HTML главной не нашлось /_next/image?url=%2Flanding%2F... — экстракция сломана или лендинг пуст.");
  process.exit(1);
}
console.log(`Прогрев ${urls.length} URL оптимизатора на ${BASE_URL} (×${CONCURRENCY} воркеров)`);

let warm = 0;
const failed = [];
let done = 0;
async function warmOne(path) {
  const started = Date.now();
  const res = await fetchWithRetry(`${BASE_URL}${path}`, { attempts: RETRIES, accept: true });
  done += 1;
  if (res) {
    warm += 1;
    console.log(`[${done}/${urls.length}] ✓ ${path} (${((Date.now() - started) / 1000).toFixed(1)}s)`);
  } else {
    failed.push(path);
    console.error(`[${done}/${urls.length}] ✗ ${path}`);
  }
}

// Пул на CONCURRENCY воркеров: каждый берёт следующий URL из очереди.
let next = 0;
await Promise.all(
  Array.from({ length: Math.min(CONCURRENCY, urls.length) }, async () => {
    while (next < urls.length) {
      const path = urls[next++];
      await warmOne(path);
    }
  }),
);

console.log(`\nИтог: ${warm}/${urls.length} прогрето`);
if (failed.length > 0) {
  console.error(`Не прогрелись (${failed.length}):`);
  for (const f of failed) console.error(`  ${f}`);
  process.exit(1);
}
