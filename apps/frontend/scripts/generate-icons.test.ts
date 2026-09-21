import { readFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import sharp from 'sharp';
import { buildIco } from './lib/png-ico.mjs';

// Pixel-contract tests for the PWA icon set (issue #781, research
// docs/research/pwa-icons-canon.md). The generator bakes the corner rounding
// into any/favicon icons (browser tabs and desktop PWA icons are shown as-is,
// no mask), while maskable / apple-touch must stay opaque full squares
// (Android launchers and iOS apply their own masks and would surface
// transparent corners as black holes). These tests pin that contract against
// the committed artifacts, so a generator regression cannot land silently.

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

async function cornerAndCenterPixels(
    file: string,
): Promise<{ cornerAlpha: number; centerAlpha: number; width: number; height: number }> {
    const buf = await readFile(resolve(frontendRoot, file));
    const { data, info } = await sharp(buf).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
    const px = (x: number, y: number): number =>
        data.readUInt8((y * info.width + x) * info.channels + info.channels - 1);
    return {
        cornerAlpha: px(0, 0),
        centerAlpha: px(Math.floor(info.width / 2), Math.floor(info.height / 2)),
        width: info.width,
        height: info.height,
    };
}

/** Minimal reader for the single-image 32bpp BMP-in-ICO produced by buildIco. */
async function icoPixel(file: string, x: number, y: number): Promise<number[]> {
    const buf = await readFile(resolve(frontendRoot, file));
    const imageOffset = buf.readUInt32LE(18);
    const bmp = buf.subarray(imageOffset);
    const width = bmp.readUInt32LE(4);
    const storedHeight = bmp.readUInt32LE(8);
    expect(bmp.readUInt32LE(0)).toBe(40); // BITMAPINFOHEADER size
    expect(bmp.readUInt16LE(14)).toBe(32); // bit count
    expect(storedHeight).toBe(width * 2); // XOR + AND masks
    const realHeight = storedHeight / 2;
    // XOR mask is bottom-up BGRA; AND mask (realHeight rows, 4-byte aligned) follows.
    const andMaskOffset = 40 + width * realHeight * 4;
    const row = realHeight - 1 - y;
    const i = 40 + (row * width + x) * 4;
    const bgra = [
        bmp.readUInt8(i + 2),
        bmp.readUInt8(i + 1),
        bmp.readUInt8(i),
        bmp.readUInt8(i + 3),
    ];
    const andRowSize = Math.ceil(width / 32) * 4;
    const andByte = bmp.readUInt8(andMaskOffset + row * andRowSize + (x >> 3));
    // AND bit set (1) means transparent — must be clear for alpha-driven icons.
    expect((andByte >> (7 - (x % 8))) & 1).toBe(0);
    return bgra;
}

describe('png-ico builder', () => {
    it('wraps raw RGBA into a single-image 32bpp BMP-in-ICO', () => {
        // 2×2: red top-left, green top-right, translucent blue bottom-left, white bottom-right.
        const rgba = Buffer.from([
            255, 0, 0, 255, 0, 255, 0, 255,
            0, 0, 255, 128, 255, 255, 255, 255,
        ]);
        const ico = buildIco({ width: 2, height: 2, rgba });

        expect(ico.readUInt16LE(0)).toBe(0); // reserved
        expect(ico.readUInt16LE(2)).toBe(1); // type: icon
        expect(ico.readUInt16LE(4)).toBe(1); // one image
        expect(ico.readUInt8(6)).toBe(2); // width
        expect(ico.readUInt8(7)).toBe(2); // height
        expect(ico.readUInt16LE(10)).toBe(1); // planes
        expect(ico.readUInt16LE(12)).toBe(32); // bpp
        const bytesInRes = ico.readUInt32LE(14);
        const imageOffset = ico.readUInt32LE(18);
        expect(imageOffset).toBe(22); // ICONDIR + ICONDIRENTRY
        expect(ico.length).toBe(imageOffset + bytesInRes);

        const bmp = ico.subarray(imageOffset);
        expect(bmp.readUInt32LE(0)).toBe(40); // BITMAPINFOHEADER size
        expect(bmp.readInt32LE(4)).toBe(2); // width
        expect(bmp.readInt32LE(8)).toBe(4); // stored height: XOR + AND
        expect(bmp.readUInt16LE(12)).toBe(1); // planes
        expect(bmp.readUInt16LE(14)).toBe(32); // bpp
        expect(bmp.readUInt32LE(16)).toBe(0); // BI_RGB, uncompressed
        expect(bmp.readUInt32LE(20)).toBe(2 * 2 * 4 + 8); // biSizeImage: XOR + AND masks
        // Bottom-up rows: first stored row is the bottom image row, BGRA order.
        expect([...bmp.subarray(40, 44)]).toEqual([255, 0, 0, 128]); // blue, alpha kept
        expect([...bmp.subarray(44, 48)]).toEqual([255, 255, 255, 255]); // white
        expect([...bmp.subarray(48, 52)]).toEqual([0, 0, 255, 255]); // red on top row
        expect([...bmp.subarray(52, 56)]).toEqual([0, 255, 0, 255]); // green
        // AND mask (8 bytes for a 2×2 icon) is all clear — alpha channel decides.
        expect([...bmp.subarray(56, 64)]).toEqual([0, 0, 0, 0, 0, 0, 0, 0]);
        expect(bytesInRes).toBe(64);
    });
});

describe('any + favicon icons keep transparent rounded corners', () => {
    it.each([
        ['public/icons/icon-192.png', 192],
        ['public/icons/icon-512.png', 512],
        ['app/icon.png', 512],
    ])('%s is %ix%i with transparent corners and an opaque body', async (file, size) => {
        const px = await cornerAndCenterPixels(file);
        expect(px.width).toBe(size);
        expect(px.height).toBe(size);
        expect(px.cornerAlpha).toBe(0);
        expect(px.centerAlpha).toBe(255);
    });

    it('app/favicon.ico is a 32×32 icon with transparent corners', async () => {
        const buf = await readFile(resolve(frontendRoot, 'app/favicon.ico'));
        expect(buf.readUInt16LE(2)).toBe(1); // type: icon
        expect(buf.readUInt16LE(4)).toBe(1); // one image
        expect(buf.readUInt8(6)).toBe(32);
        expect(buf.readUInt8(7)).toBe(32);
        const corner = await icoPixel('app/favicon.ico', 0, 0);
        const center = await icoPixel('app/favicon.ico', 16, 16);
        expect(corner[3]).toBe(0); // alpha
        expect(center[3]).toBe(255);
    });
});

describe('maskable + apple-touch icons stay opaque full squares', () => {
    it('public/icons/icon-maskable-512.png has no transparent pixels in the corners', async () => {
        const px = await cornerAndCenterPixels('public/icons/icon-maskable-512.png');
        expect(px.width).toBe(512);
        expect(px.height).toBe(512);
        expect(px.cornerAlpha).toBe(255);
        expect(px.centerAlpha).toBe(255);
    });

    it('app/apple-icon.png is a 180×180 opaque square (iOS paints alpha black)', async () => {
        const px = await cornerAndCenterPixels('app/apple-icon.png');
        expect(px.width).toBe(180);
        expect(px.height).toBe(180);
        expect(px.cornerAlpha).toBe(255);
        expect(px.centerAlpha).toBe(255);
    });
});

describe('monochrome silhouette stays a tintable alpha mask', () => {
    it('public/icons/icon-monochrome-512.png is 512×512 on transparent background', async () => {
        const px = await cornerAndCenterPixels('public/icons/icon-monochrome-512.png');
        expect(px.width).toBe(512);
        expect(px.height).toBe(512);
        expect(px.cornerAlpha).toBe(0);
        expect(px.centerAlpha).toBe(255); // house body — the tintable silhouette
    });
});
