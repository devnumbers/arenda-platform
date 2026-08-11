/*
 * Generates iOS PWA splash images (apple-touch-startup-image) for current
 * iPhone/iPad device pixel sizes from the brand icon, and writes the matching
 * <link> tag descriptors to shared/lib/pwa/splash-manifest.json so layout.tsx
 * can import them without hand-sync.
 *
 * Run:  npm run generate:splash
 *
 * Output:
 *   public/icons/splash/<w>x<h>.png            — one PNG per portrait/landscape size
 *   shared/lib/pwa/splash-manifest.json        — { href, media }[] consumed by layout.tsx
 *
 * Design: solid #ffffff background (matches manifest background_color), brand
 * logo centered and scaled to 28% of the shorter canvas side so it reads well
 * on every device without edge clipping. iOS does NOT scale a single splash
 * image — it matches exact device pixels, so each variant is mandatory.
 *
 * Regenerate after changing the brand icon or the device matrix below. The PNGs
 * and the manifest are committed (not built on `next build`).
 *
 * Device matrix sourced from Apple Human Interface Guidelines device sizes and
 * the community-maintained apple-touch-startup-image media query list
 * (Evan Bacon gist + Spomky-Labs PWA bundle), extended for Dynamic Island
 * devices (iPhone 14/15/16 Pro / Pro Max). Coverage is best-effort for current
 * devices, not exhaustive; add new Apple screen sizes here as they ship.
 */

import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import sharp from 'sharp';

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(__dirname, '..');
const iconPath = resolve(frontendRoot, 'public/icons/icon-512.png');
const outDir = resolve(frontendRoot, 'public/icons/splash');

// Brand colors — must match manifest background_color / theme_color.
const BACKGROUND_COLOR = '#ffffff';

// Logo scale relative to the SHORTER canvas dimension. 0.28 keeps the house
// logo readable on small (SE) and large (iPad Pro 12.9) screens alike without
// touching the edges under the safe area.
const LOGO_SCALE = 0.28;

/*
 * iOS device splash pixel dimensions (portrait width × height) and their media
 * queries. Each entry generates a portrait and a landscape PNG (dimensions
 * swapped). The media query targets the LOGICAL device-width/device-height plus
 * the -webkit-device-pixel-ratio, so the same query works for both orientations
 * via the `orientation:` clause.
 *
 * `w`/`h` below are the PHYSICAL pixel dimensions of the portrait splash PNG.
 */
const DEVICES = [
  // iPhone — physical portrait pixels
  { name: 'iphone-se', w: 640, h: 1136, dw: 320, dh: 568, dpr: 2 },
  { name: 'iphone-8', w: 750, h: 1334, dw: 375, dh: 667, dpr: 2 },
  { name: 'iphone-8-plus', w: 1242, h: 2208, dw: 414, dh: 736, dpr: 3 },
  { name: 'iphone-x-11pro-12mini', w: 1125, h: 2436, dw: 375, dh: 812, dpr: 3 },
  { name: 'iphone-11-xr', w: 828, h: 1792, dw: 414, dh: 896, dpr: 2 },
  { name: 'iphone-12-13-14', w: 1170, h: 2532, dw: 390, dh: 844, dpr: 3 },
  { name: 'iphone-12-13-14-plus', w: 1284, h: 2778, dw: 428, dh: 926, dpr: 3 },
  { name: 'iphone-14-15-16-pro', w: 1179, h: 2556, dw: 393, dh: 852, dpr: 3 },
  { name: 'iphone-14-15-16-pro-max', w: 1290, h: 2796, dw: 430, dh: 932, dpr: 3 },
  // iPad — physical portrait pixels
  { name: 'ipad-mini-6', w: 1536, h: 2048, dw: 768, dh: 1024, dpr: 2 },
  { name: 'ipad-10', w: 1640, h: 2360, dw: 820, dh: 1180, dpr: 2 },
  { name: 'ipad-pro-11', w: 1668, h: 2388, dw: 834, dh: 1194, dpr: 2 },
  { name: 'ipad-pro-10-5', w: 1668, h: 2224, dw: 834, dh: 1112, dpr: 2 },
  { name: 'ipad-pro-12-9', w: 2048, h: 2732, dw: 1024, dh: 1366, dpr: 2 },
];

async function renderSplash(icon, w, h) {
  const compositeLayer = {
    input: (await icon.clone().resize({
      width: Math.round(Math.min(w, h) * LOGO_SCALE),
      height: Math.round(Math.min(w, h) * LOGO_SCALE),
      fit: 'contain',
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    }).png().toBuffer()),
    gravity: 'center',
  };
  return sharp({
    create: {
      width: w,
      height: h,
      channels: 4,
      background: BACKGROUND_COLOR,
    },
  })
    .composite([compositeLayer])
    .png();
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const icon = sharp(await readFile(iconPath));

  const links = [];
  for (const device of DEVICES) {
    for (const orientation of ['portrait', 'landscape']) {
      const isPortrait = orientation === 'portrait';
      const w = isPortrait ? device.w : device.h;
      const h = isPortrait ? device.h : device.w;
      const file = `splash-${w}x${h}.png`;
      const abs = resolve(outDir, file);
      await (await renderSplash(icon, w, h)).toFile(abs);

      const dw = device.dw;
      const dh = device.dh;
      const dpr = device.dpr;
      const media =
        `screen and (device-width: ${dw}px) and (device-height: ${dh}px) ` +
        `and (-webkit-device-pixel-ratio: ${dpr}) and (orientation: ${orientation})`;
      links.push({
        href: `/icons/splash/${file}`,
        media,
        w,
        h,
        orientation,
      });
    }
  }

  // Emit the link tag list as JSON so layout.tsx can import it without hand-sync.
  const manifestPath = resolve(frontendRoot, 'shared/lib/pwa/splash-manifest.json');
  await mkdir(dirname(manifestPath), { recursive: true });
  await writeFile(
    manifestPath,
    JSON.stringify(
      links.map(({ href, media }) => ({ href, media })),
      null,
      2,
    ) + '\n',
  );

  console.log(
    `Generated ${links.length} splash images in public/icons/splash/ ` +
      `and manifest at shared/lib/pwa/splash-manifest.json`,
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
