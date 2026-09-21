/*
 * Generates the PWA icon set for the cabinet app from the Figma vector
 * masters, covering Android (any / maskable / monochrome), iOS
 * (apple-touch-icon) and the in-tab favicon (issue #781; platform matrix in
 * docs/research/pwa-icons-canon.md).
 *
 * Run:  npm run generate:icons
 *
 * Sources (exported from Figma «Рентли. Новые экраны сервиса», App Icon
 * frames — round-corner 2358:139864 and its square copy 2353:52637):
 *   assets/icons/source-icon.svg          — rounded-square master, 1024×1024
 *                                            viewBox. Light-blue #BCDCFF
 *                                            rounded background with the blue
 *                                            #2B7FFF house; the area outside
 *                                            the rounded shape is transparent.
 *                                            Browser tabs and desktop PWA
 *                                            icons (Windows taskbar, macOS
 *                                            dock) are composited as-is — no
 *                                            mask — so the rounding must be
 *                                            baked into the pixels with
 *                                            transparent corners.
 *   assets/icons/source-square.svg        — the same artwork on an opaque full
 *                                            square. Android launchers and iOS
 *                                            apply their own masks; transparent
 *                                            corners would surface as black
 *                                            (iOS) or white/holes (Android)
 *                                            there, so these outputs must stay
 *                                            fully opaque.
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
 *   app/favicon.ico                       — Next.js file convention → served at
 *                                            /favicon.ico for crawlers and tools
 *                                            that ignore <link> (32×32, rounded
 *                                            corners stay transparent)
 *   app/apple-icon.png                    — Next.js file convention → <link rel="apple-touch-icon">
 *
 * Design notes:
 *   - Purpose "any maskable" is deliberately never combined (see manifest.ts):
 *     desktop Chrome/macOS would pick the square maskable file for shortcuts
 *     and dock icons (Chromium 40827667).
 *   - The SVG masters rasterize at 2048×2048 (2× supersample) and every output
 *     downscales from that with Lanczos3, so small sizes keep smooth edges.
 *   - PNGs are committed (not built on `next build`), same convention as the
 *     splash generator. Re-run after replacing the source assets.
 */

import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import sharp from 'sharp';
import { buildIco } from './lib/png-ico.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, '..');

const sourceIconPath = resolve(frontendRoot, 'assets/icons/source-icon.svg');
const sourceSquarePath = resolve(frontendRoot, 'assets/icons/source-square.svg');
const sourceMonochromePath = resolve(frontendRoot, 'assets/icons/source-monochrome.svg');

const publicIconsDir = resolve(frontendRoot, 'public/icons');
const appDir = resolve(frontendRoot, 'app');

// 2× the largest output (512) — rasterized once per master, downscaled below.
const RENDER_SIZE = 2048;

/**
 * Rasterizes an SVG master at RENDER_SIZE×RENDER_SIZE with the alpha channel
 * intact. Downstream variants only resize this raster.
 */
async function renderMaster(svgPath) {
  const svg = await readFile(svgPath);
  return sharp(svg, { density: (72 * RENDER_SIZE) / 1024 })
    .resize({ width: RENDER_SIZE, height: RENDER_SIZE, fit: 'fill', kernel: 'lanczos3' })
    .png()
    .toBuffer();
}

/**
 * Resizes a PNG buffer to `size`×`size` with high-quality Lanczos3 and
 * returns the PNG buffer.
 */
async function resizeTo(src, size) {
  return sharp(src).resize({
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
  const svg = await readFile(sourceMonochromePath);
  return sharp(svg, { density: (72 * size) / 512 })
    .resize({
      width: size,
      height: size,
      fit: 'fill',
    })
    .png()
    .toBuffer();
}

/**
 * Wraps a square PNG into a single-image 32bpp .ico (see png-ico.mjs).
 */
async function pngToIco(png, size) {
  const { data, info } = await sharp(png).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  if (info.width !== size || info.height !== size) {
    throw new Error(`expected ${size}×${size} raster, got ${info.width}×${info.height}`);
  }
  return buildIco({ width: size, height: size, rgba: data });
}

async function write(buf, absPath, label) {
  await sharp(buf).toFile(absPath);
  console.log(`  ✓ ${label}  →  ${absPath.replace(frontendRoot + '/', '')}`);
}

async function main() {
  // Rounded master: tabs and desktop shortcuts are shown as-is, so the corner
  // rounding ships as baked pixels with transparent corners.
  const rounded = await renderMaster(sourceIconPath);
  // Square master: maskable / apple-touch must be opaque full squares — their
  // consumers mask the shape themselves and paint alpha gaps black/white.
  const square = await renderMaster(sourceSquarePath);

  // manifest purpose: any (installable — Chrome requires 192 + 512).
  const icon192 = await resizeTo(rounded, 192);
  const icon512 = await resizeTo(rounded, 512);

  // manifest purpose: maskable (Android Adaptive Icons). Opaque square from
  // the square master; launchers pick the mask shape.
  const maskable512 = await resizeTo(square, 512);

  // Next.js file conventions for favicon + apple-touch-icon + /favicon.ico.
  const favicon512 = await resizeTo(rounded, 512);
  const faviconIco = await pngToIco(await resizeTo(rounded, 32), 32);
  const appleIcon180 = await resizeTo(square, 180);

  // manifest purpose: monochrome (Android 13+ Material You themed icons).
  const monochrome512 = await renderMonochrome(512);

  await write(icon192, resolve(publicIconsDir, 'icon-192.png'), 'icon-192          (any, installable)');
  await write(icon512, resolve(publicIconsDir, 'icon-512.png'), 'icon-512          (any, installable)');
  await write(maskable512, resolve(publicIconsDir, 'icon-maskable-512.png'), 'icon-maskable-512 (maskable, opaque square)');
  await write(monochrome512, resolve(publicIconsDir, 'icon-monochrome-512.png'), 'icon-monochrome   (monochrome silhouette)');
  await write(favicon512, resolve(appDir, 'icon.png'), 'app/icon.png      (favicon, <link rel="icon">)');
  await writeFile(resolve(appDir, 'favicon.ico'), faviconIco);
  console.log('  ✓ favicon.ico 32   →  app/favicon.ico');
  await write(appleIcon180, resolve(appDir, 'apple-icon.png'), 'app/apple-icon.png (<link rel="apple-touch-icon">)');

  console.log(
    '\nGenerated 8 icon files. Run `npm run generate:splash` to refresh ' +
      'iOS splash images against the new icon-512.png.',
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
