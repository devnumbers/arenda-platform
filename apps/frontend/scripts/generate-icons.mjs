/*
 * Generates the PWA icon set for the cabinet app from the glass master asset,
 * covering Android (any / maskable / monochrome) and iOS (apple-touch-icon),
 * plus the in-tab favicon.
 *
 * Run:  npm run generate:icons
 *
 * Sources:
 *   assets/icons/source-icon.png          — glass master, 1024×1024 RGBA.
 *                                            Round-corner PNG (transparent
 *                                            corners) — the generator fills the
 *                                            corners with FILL_COLOR so the
 *                                            result is an opaque square, which
 *                                            is mandatory for maskable icons
 *                                            (transparent corners show up as
 *                                            black/white holes after the
 *                                            launcher applies its mask) and
 *                                            safe for iOS apple-touch-icon
 *                                            (iOS would otherwise paint the
 *                                            alpha gaps black).
 *   assets/icons/source-monochrome.svg    — house silhouette only, white on
 *                                            transparent. Used for the Android
 *                                            13+ Material You "monochrome"
 *                                            purpose: the system tints this
 *                                            alpha mask with the wallpaper
 *                                            color, so glass / gradients from
 *                                            the colored master cannot survive.
 *
 * Output:
 *   public/icons/icon-192.png             — manifest purpose: any (installable)
 *   public/icons/icon-512.png             — manifest purpose: any (installable;
 *                                            also the source for generate-splash)
 *   public/icons/icon-maskable-512.png    — manifest purpose: maskable
 *                                            (Android Adaptive Icons)
 *   public/icons/icon-monochrome-512.png  — manifest purpose: monochrome
 *                                            (Android 13+ themed icons)
 *   app/icon.png                          — Next.js file convention → <link rel="icon">
 *   app/apple-icon.png                    — Next.js file convention → <link rel="apple-touch-icon">
 *
 * Design notes:
 *   - The glass master already keeps all meaningful content (the house mark)
 *     well inside the central 80% safe zone required by maskable icons
 *     (web.dev/articles/maskable-icon: circle of diameter 80%). No padding is
 *     added — the master is used as authored.
 *   - FILL_COLOR matches the master's light-blue edge so the corner fill is
 *     visually continuous with the existing background, not a hard frame.
 *   - PNGs are committed (not built on `next build`), same convention as the
 *     splash generator. Re-run after replacing the source assets.
 */

import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import sharp from 'sharp';

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, '..');

const sourceIconPath = resolve(frontendRoot, 'assets/icons/source-icon.png');
const sourceMonochromePath = resolve(frontendRoot, 'assets/icons/source-monochrome.svg');

const publicIconsDir = resolve(frontendRoot, 'public/icons');
const appDir = resolve(frontendRoot, 'app');

// Light blue that matches the glass master's edge — keeps the corner fill
// continuous with the existing background instead of introducing a frame.
const FILL_COLOR = '#bcdcff';

/**
 * Loads the glass master as an opaque 1024×1024 square: flattens the alpha
 * channel onto FILL_COLOR so the transparent round corners become solid.
 * Every downstream `any`/`maskable`/`apple-touch` variant derives from this.
 */
async function loadOpaqueMaster() {
  const buf = await readFile(sourceIconPath);
  return sharp(buf).flatten({ background: FILL_COLOR }).png();
}

/**
 * Resizes a sharp pipeline to `size`×`size` with high-quality Lanczos3 and
 * returns the PNG buffer.
 */
async function resizeTo(src, size) {
  return src.clone().resize({
    width: size,
    height: size,
    fit: 'fill',
    kernel: 'lanczos3',
  }).png().toBuffer();
}

/**
 * Renders the monochrome silhouette PNG at `size`×`size`: rasterizes the SVG
 * (white house on transparent) and keeps the alpha channel intact so the
 * system can tint it. No flatten — transparency is required here.
 */
async function renderMonochrome(size) {
  const svg = await readFile(sourceMonochromePath, 'utf8');
  return sharp(Buffer.from(svg))
    .resize({
      width: size,
      height: size,
      fit: 'fill',
    })
    .png()
    .toBuffer();
}

async function write(buf, absPath, label) {
  await sharp(buf).toFile(absPath);
  console.log(`  ✓ ${label}  →  ${absPath.replace(frontendRoot + '/', '')}`);
}

async function main() {
  const master = await loadOpaqueMaster();

  // manifest purpose: any (installable — Chrome requires 192 + 512).
  const icon192 = await resizeTo(master, 192);
  const icon512 = await resizeTo(master, 512);

  // manifest purpose: maskable (Android Adaptive Icons). Same opaque square
  // as `any`; the house mark is already inside the 80% safe zone.
  const maskable512 = icon512;

  // Next.js file conventions for favicon + apple-touch-icon.
  const favicon128 = await resizeTo(master, 128);
  const appleIcon180 = await resizeTo(master, 180);

  // manifest purpose: monochrome (Android 13+ Material You themed icons).
  const monochrome512 = await renderMonochrome(512);

  await write(icon192, resolve(publicIconsDir, 'icon-192.png'), 'icon-192         (any, installable)');
  await write(icon512, resolve(publicIconsDir, 'icon-512.png'), 'icon-512         (any, installable)');
  await write(maskable512, resolve(publicIconsDir, 'icon-maskable-512.png'), 'icon-maskable-512 (maskable)');
  await write(monochrome512, resolve(publicIconsDir, 'icon-monochrome-512.png'), 'icon-monochrome   (monochrome silhouette)');
  await write(favicon128, resolve(appDir, 'icon.png'), 'app/icon.png      (favicon, <link rel="icon">)');
  await write(appleIcon180, resolve(appDir, 'apple-icon.png'), 'app/apple-icon.png (<link rel="apple-touch-icon">)');

  console.log(
    '\nGenerated 7 icon files. Update app/manifest.ts to reference maskable + ' +
      'monochrome if not done yet. Run `npm run generate:splash` to refresh ' +
      'iOS splash images against the new icon-512.png.',
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
