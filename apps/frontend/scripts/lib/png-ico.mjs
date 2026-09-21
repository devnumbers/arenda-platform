/**
 * Builds a single-image, uncompressed 32bpp Windows .ico container around raw
 * RGBA pixels (BMP-in-ICO). Chosen over PNG-in-ICO and over external tooling
 * so the favicon stays dependency-free and parses everywhere, including legacy
 * favicon.ico consumers that request /favicon.ico without looking at <link>.
 *
 * Layout: ICONDIR (6 bytes) + ICONDIRENTRY (16 bytes) + BITMAPINFOHEADER (40)
 * + XOR mask (bottom-up BGRA rows) + AND mask (1 bit/px, rows padded to 32
 * bits, all clear — the alpha channel decides transparency).
 */

/**
 * @param {{ width: number, height: number, rgba: Buffer}} image
 * @returns {Buffer} the .ico file contents
 */
export function buildIco({ width, height, rgba }) {
  if (rgba.length !== width * height * 4) {
    throw new Error(`rgba must be ${width}×${height}×4 bytes, got ${rgba.length}`);
  }

  const andRowSize = Math.ceil(width / 32) * 4;
  const andMaskSize = andRowSize * height;
  const imageSize = 40 + width * height * 4 + andMaskSize;
  const ico = Buffer.alloc(22 + imageSize);

  // ICONDIR: reserved, type 1 (icon), one image.
  ico.writeUInt16LE(0, 0);
  ico.writeUInt16LE(1, 2);
  ico.writeUInt16LE(1, 4);
  // ICONDIRENTRY: 0 in the width/height byte means 256, otherwise the size.
  ico.writeUInt8(width === 256 ? 0 : width, 6);
  ico.writeUInt8(height === 256 ? 0 : height, 7);
  ico.writeUInt8(0, 8); // color count (palette icons only)
  ico.writeUInt8(0, 9); // reserved
  ico.writeUInt16LE(1, 10); // planes
  ico.writeUInt16LE(32, 12); // bits per pixel
  ico.writeUInt32LE(imageSize, 14);
  ico.writeUInt32LE(22, 18); // image offset

  const bmp = ico.subarray(22);
  bmp.writeUInt32LE(40, 0); // BITMAPINFOHEADER size
  bmp.writeInt32LE(width, 4);
  bmp.writeInt32LE(height * 2, 8); // XOR mask + AND mask
  bmp.writeUInt16LE(1, 12); // planes
  bmp.writeUInt16LE(32, 14); // bits per pixel
  bmp.writeUInt32LE(0, 16); // BI_RGB, uncompressed
  bmp.writeUInt32LE(width * height * 4 + andMaskSize, 20);

  // XOR mask: bottom-up rows, BGRA byte order.
  for (let y = 0; y < height; y += 1) {
    const srcRow = (height - 1 - y) * width * 4;
    const dstRow = 40 + y * width * 4;
    for (let x = 0; x < width; x += 1) {
      const s = srcRow + x * 4;
      const d = dstRow + x * 4;
      bmp[d] = rgba[s + 2];
      bmp[d + 1] = rgba[s + 1];
      bmp[d + 2] = rgba[s];
      bmp[d + 3] = rgba[s + 3];
    }
  }
  // AND mask stays all clear: for 32bpp icons the alpha channel is authoritative.

  return ico;
}
