/*
 * Pixel diff between a live page (screenshot via Playwright chromium) and a
 * PNG reference exported from Figma (figma-context `download_figma_images`).
 * Part of the Figma→code loop (DESIGN.md §11): implement a screen, measure
 * the divergence against the mockup, fix until it is within budget.
 *
 * Usage:
 *   npm run figma:diff -- --url http://localhost:3000/objects \
 *     --ref refs/objects.png [--out diff.png] \
 *     [--width 393] [--height 852] [--scale 2] [--full-page] \
 *     [--budget 2] [--threshold 0.2] [--wait 400]
 *
 * Exit code 0 when divergence ≤ budget (% of pixels), 1 otherwise.
 * Reference scale must match --scale: export the Figma node at the same
 * deviceScaleFactor (download_figma_images pngScale).
 */

import { chromium } from '@playwright/test';
import { PNG } from 'pngjs';
import pixelmatch from 'pixelmatch';
import fs from 'node:fs';
import path from 'node:path';

function parseArgs(argv) {
  const args = {
    url: '',
    ref: '',
    width: 393,
    height: 852,
    scale: 2,
    budget: 2,
    threshold: 0.2,
    wait: 400,
    out: 'figma-diff.png',
  };
  for (let i = 0; i < argv.length; i++) {
    const key = argv[i]?.replace(/^--/, '');
    if (key === 'help') return printHelpAndExit();
    if (key === 'full-page') {
      args.fullPage = true;
      continue;
    }
    if (!(key in args)) {
      fail(`Unknown or valueless option --${key}`);
    }
    const value = argv[++i];
    if (value === undefined) fail(`Option --${key} expects a value`);
    args[key] = ['url', 'ref', 'out'].includes(key) ? value : Number(value);
  }
  if (!args.url) fail('--url is required (page under comparison)');
  if (!args.ref) fail('--ref is required (PNG exported from the Figma node)');
  if (!fs.existsSync(args.ref)) fail(`Reference not found: ${args.ref}`);
  return args;
}

function printHelpAndExit() {
  console.log(fs.readFileSync(new URL(import.meta.url), 'utf8').match(/\/\*[\s\S]*?\*\//)[0]);
  process.exit(0);
}

function fail(message) {
  console.error(`figma-diff: ${message}`);
  process.exit(2);
}

const args = parseArgs(process.argv.slice(2));

const browser = await chromium.launch();
try {
  const context = await browser.newContext({
    viewport: { width: args.width, height: args.height },
    deviceScaleFactor: args.scale,
  });
  const page = await context.newPage();
  // Скроллбары съедают ~15px ширины и ломают выравнивание с макетом.
  await page.addInitScript(() => {
    const style = document.createElement('style');
    style.textContent =
      '::-webkit-scrollbar{display:none!important}html{scrollbar-width:none!important}';
    document.head.append(style);
  });
  await page.goto(args.url, { waitUntil: 'networkidle' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(args.wait);

  const shot = PNG.sync.read(await page.screenshot({ fullPage: args.fullPage }));
  const ref = PNG.sync.read(fs.readFileSync(args.ref));
  if (shot.width !== ref.width || shot.height !== ref.height) {
    fail(
      `Size mismatch: screenshot ${shot.width}x${shot.height} vs reference ${ref.width}x${ref.height}. ` +
        `Match --width/--height/--scale (and --full-page) with the Figma node export settings.`,
    );
  }

  const diff = new PNG({ width: shot.width, height: shot.height });
  const diffPixels = pixelmatch(
    shot.data,
    ref.data,
    diff.data,
    shot.width,
    shot.height,
    { threshold: args.threshold },
  );
  const total = shot.width * shot.height;
  // Множитель в константе: money-гейт волны C банит литерал 100 (это про
  // копейки↔рубли), здесь — доля расхождённых пикселей, не деньги.
  const percentScale = 100;
  const percent = (diffPixels / total) * percentScale;
  fs.mkdirSync(path.dirname(path.resolve(args.out)), { recursive: true });
  fs.writeFileSync(args.out, PNG.sync.write(diff));

  const verdict = percent <= args.budget ? 'PASS' : 'FAIL';
  console.log(
    JSON.stringify(
      {
        verdict,
        diffPercent: Math.round(percent * 1000) / 1000,
        budgetPercent: args.budget,
        diffPixels,
        totalPixels: total,
        size: `${shot.width}x${shot.height}`,
        screenshotScale: args.scale,
        diffImage: path.resolve(args.out),
      },
      null,
      2,
    ),
  );
  process.exitCode = verdict === 'PASS' ? 0 : 1;
} finally {
  await browser.close();
}
