#!/usr/bin/env node
// Прогрев кэша image-оптимизатора лендинга (Next /_next/image). Оптимизатор
// кеширует результат на диске (.next/cache/images), но контейнер деплоя
// стартует с холодным кэшем: первый посетитель каждой картинки платит за
// sharp-оптимизацию. Скрипт после деплоя (или локально перед приёмкой)
// обходит страницы, собирает все ссылки /_next/image?… из src/srcset
// (srcset уже перечислен в разметке — покрываем ровно те варианты, что
// запросят браузеры) и GET-ит каждый с современным
// Accept — кэш тёплый до первого живого посетителя.
//
// Строго последовательно, с паузами и ретраями с бэкоффом в том числе на
// HTTP-ошибки (решение владельца 15.10 по прогону стейджа): параллельная
// заливка холодных 4k AVIF @ q95 валила контейнер лендинга — 8 потоков
// дали 18/134 (Caddy массово 502, апстрим OOM/рестарт), 2 потока — стену
// 502 после ~40-й картинки (41/134), причём падали и мгновенные иконки.
// Одиночное 4k-кодирование контейнер держит спокойно (~2–3 с), поэтому
// Sequence + бэкофф — рабочий профиль: тот же прогон последовательно дал
// 134/134 без единого провала.
//
// Запуск: node scripts/warm-images.mjs [baseUrl]
//   node scripts/warm-images.mjs https://dev.rentlee.ru
//   node scripts/warm-images.mjs            # дефолт http://localhost:3000
//
// Возврат: 0 если всё согрелось, 1 если есть неудачи (для CI).

const BASE = (process.argv[2] ?? "http://localhost:3000").replace(/\/$/, "");
const PAGES = ["/", "/privacy", "/terms"];
const ACCEPT =
  "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8";
const TIMEOUT_MS = 120000;
const GAP_MS = 500;
const MAX_ATTEMPTS = 5;
const BACKOFF_MS = [5000, 15000, 30000, 30000];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function extractImageUrls(html) {
  const urls = new Set();
  // Атрибуты src и srcset у <img>; srcset — через запятую с дескрипторами.
  // Экранирование разворачиваем обратно: HTML (&amp;) и JSON инлайн-скриптов
  // RSC (\u0026) — оба варианта встречаются в выдаче (media-ветки <picture>
  // живут в самовставляемом flight-_payload).
  const pattern = /\/_next\/image\?[^"'\s,)]+/g;
  for (const match of html.matchAll(pattern)) {
    const url = match[0]
      .replaceAll("&amp;", "&")
      .replaceAll("\\u0026", "&");
    urls.add(url);
  }
  return [...urls];
}

const pages = [];
for (const path of PAGES) {
  const res = await fetch(new URL(path, BASE), {
    headers: { Accept: "text/html" },
    signal: AbortSignal.timeout(TIMEOUT_MS),
  });
  if (!res.ok) {
    console.error(`Страница ${path} ответила ${res.status} — прогрев отменён`);
    process.exit(1);
  }
  pages.push(await res.text());
}

const urls = [...new Set(pages.flatMap(extractImageUrls))];
console.log(`${BASE}: ${urls.length} уникальных вариантов картинок, грею…`);

let index = 0;
let hits = 0;
const failed = [];

for (const url of urls) {
  index += 1;
  let ok = false;
  let hit = false;
  let last = "";
  for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt += 1) {
    try {
      const res = await fetch(new URL(url, BASE), {
        headers: { Accept: ACCEPT },
        signal: AbortSignal.timeout(TIMEOUT_MS),
      });
      // Тело нужно дочитать — иначе соединение не освободится и кэш
      // может не успеть записаться до закрытия потока.
      await res.arrayBuffer();
      if (res.ok) {
        ok = true;
        hit = res.headers.get("x-nextjs-cache") === "HIT";
        break;
      }
      // 502 от Caddy — апстрим переживает рестарт/затишье: не выходить
      // сразу, подождать и попробовать снова.
      last = `HTTP ${res.status}`;
    } catch (error) {
      last = String(error).slice(0, 100);
    }
    if (attempt < MAX_ATTEMPTS - 1) {
      await sleep(BACKOFF_MS[attempt] ?? BACKOFF_MS[BACKOFF_MS.length - 1]);
    }
  }
  if (ok) {
    if (hit) hits += 1;
    console.log(`[${index}/${urls.length}] ${hit ? "HIT" : "ок "} ${url}`);
  } else {
    failed.push({ url, last });
    console.error(`[${index}/${urls.length}] НЕ СОГРЕЛОСЬ [${last}] ${url}`);
  }
  await sleep(GAP_MS);
}

console.log(
  `Согрето: ${index - failed.length}/${index}` +
    (hits > 0 ? ` (из них уже в кэше: ${hits})` : ""),
);
for (const f of failed) {
  console.error(`  НЕ СОГРЕЛОСЬ [${f.last}] ${f.url}`);
}
process.exit(failed.length > 0 ? 1 : 0);
