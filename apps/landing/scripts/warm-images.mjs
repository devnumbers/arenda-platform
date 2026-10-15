#!/usr/bin/env node
// Прогрев кэша image-оптимизатора лендинга (Next /_next/image). Оптимизатор
// кеширует результат на диске (.next/cache/images), но контейнер деплоя
// стартует с холодным кэшем: первый посетитель каждой картинки платит за
// sharp-оптимизацию. Скрипт после деплоя (или локально перед приёмкой)
// обходит страницы, собирает все ссылки /_next/image?… из src/srcset
// (лестница deviceSizes/imageSizes уже перечислена в разметке — покрываем
// ровно те варианты, что запросят браузеры) и GET-ит каждый с современным
// Accept — кэш тёплый до первого живого посетителя.
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
const CONCURRENCY = 8;
const TIMEOUT_MS = 15000;
const RETRIES = 1;

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

async function warm(url) {
  for (let attempt = 0; attempt <= RETRIES; attempt += 1) {
    try {
      const res = await fetch(new URL(url, BASE), {
        headers: { Accept: ACCEPT },
        signal: AbortSignal.timeout(TIMEOUT_MS),
      });
      // Тело нужно дочитать — иначе соединение не освободится и кэш
      // может не успеть записаться до закрытия потока.
      await res.arrayBuffer();
      if (res.ok) {
        return { url, ok: true, status: res.status, cached: res.headers.get("x-nextjs-cache") };
      }
      return { url, ok: false, status: res.status };
    } catch (error) {
      if (attempt === RETRIES) {
        return { url, ok: false, status: String(error) };
      }
    }
  }
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

const results = [];
let index = 0;
async function worker() {
  while (index < urls.length) {
    const current = urls[index];
    index += 1;
    results.push(await warm(current));
  }
}
await Promise.all(Array.from({ length: CONCURRENCY }, worker));

const failed = results.filter((r) => !r.ok);
const hit = results.filter((r) => r.ok && r.cached === "HIT").length;
console.log(
  `Согрето: ${results.length - failed.length}/${results.length}` +
    (hit > 0 ? ` (из них уже в кэше: ${hit})` : ""),
);
for (const f of failed) {
  console.error(`  НЕ СОГРЕЛОСЬ [${f.status}] ${f.url}`);
}
process.exit(failed.length > 0 ? 1 : 0);
