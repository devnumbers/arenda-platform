/*
 * Builds the derivable part of the brand icon set from the committed masters —
 * the local replacement for the RealFaviconGenerator step (process removed by
 * owner decision 27.10.2026; story and mapping — assets/icons/README.md).
 * All outputs are committed; nothing is built on `next build`.
 *
 * Run:  npm run generate:icons
 *
 * Inputs (masters):
 *   assets/icons/source-icon-fitted.png       — 1024, transparent rounded corners
 *   assets/icons/source-maskable-android.png  — 1024, opaque, art inside the 80% safe zone
 *
 * Outputs:
 *   public/icons/icon-512.png                 — manifest any
 *   public/icons/icon-192.png                 — manifest any / push icon
 *   app/icon.png                              — 96, <link rel="icon">
 *   app/favicon.ico                           — multi-ICO 16/32/48 (PNG entries, transparent)
 *   public/icons/icon-maskable-512.png        — manifest maskable
 *
 * NOT built here (see README):
 *   app/apple-icon.png                    — owner's Figma export (180, opaque square)
 *   public/icons/icon-monochrome-512.png  — silhouette, changes only with its SVG master
 *   public/icons/splash/                  — npm run generate:splash (runs off the rebuilt icon-512)
 *
 * After running with a new art, bump ?v=YYYYMMDD on every icon URL consumer
 * (app/manifest.ts, shared/lib/pwa/push-payload.ts, public/sw.js) — guard:
 * shared/lib/pwa/icon-urls.test.ts. /icons/* is served immutable for a year,
 * so a new art under an unchanged URL never reaches clients.
 */

import { writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import sharp from 'sharp';

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, '..');

const fittedMaster = resolve(frontendRoot, 'assets/icons/source-icon-fitted.png');
const maskableMaster = resolve(frontendRoot, 'assets/icons/source-maskable-android.png');

const icon512 = resolve(frontendRoot, 'public/icons/icon-512.png');
const icon192 = resolve(frontendRoot, 'public/icons/icon-192.png');
const appIcon = resolve(frontendRoot, 'app/icon.png');
const favicon = resolve(frontendRoot, 'app/favicon.ico');
const maskable512 = resolve(frontendRoot, 'public/icons/icon-maskable-512.png');

// ICO carries PNG-encoded entries (Vista+); every browser reads them.
const ICO_SIZES = [16, 32, 48];

async function renderPng(masterPath, size) {
  return sharp(masterPath)
    .resize(size, size, { kernel: 'lanczos3' })
    .png()
    .toBuffer();
}

async function derivePng(masterPath, size, outPath) {
  const png = await renderPng(masterPath, size);
  await writeFile(outPath, png);
  console.log(`derived ${size}x${size}  ${outPath}`);
  return png;
}

// ICONDIR + ICONDIRENTRY per image + raw PNG payloads (specs.microsoft.com ICO format).
function packIco(entries) {
  const header = Buffer.alloc(6);
  header.writeUInt16LE(1, 2); // type: icon
  header.writeUInt16LE(entries.length, 4);

  const directory = Buffer.alloc(16 * entries.length);
  let offset = header.length + directory.length;
  const payloads = [];

  entries.forEach((entry, index) => {
    const base = index * 16;
    const sizeByte = entry.size >= 256 ? 0 : entry.size;
    directory.writeUInt8(sizeByte, base); // width
    directory.writeUInt8(sizeByte, base + 1); // height
    directory.writeUInt16LE(1, base + 4); // planes
    directory.writeUInt16LE(32, base + 6); // bits per pixel
    directory.writeUInt32LE(entry.png.length, base + 8);
    directory.writeUInt32LE(offset, base + 12);
    offset += entry.png.length;
    payloads.push(entry.png);
  });

  return Buffer.concat([header, directory, ...payloads]);
}

const icoEntries = [];
for (const size of ICO_SIZES) {
  icoEntries.push({ size, png: await renderPng(fittedMaster, size) });
}
await writeFile(favicon, packIco(icoEntries));
console.log(`packed ICO ${ICO_SIZES.join('/')}  ${favicon}`);

await derivePng(fittedMaster, 512, icon512);
await derivePng(fittedMaster, 192, icon192);
await derivePng(fittedMaster, 96, appIcon);
await derivePng(maskableMaster, 512, maskable512);
