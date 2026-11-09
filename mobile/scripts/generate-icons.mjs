// Regenerates every PNG icon asset from the vector mark below.
//
//   cd mobile && npm i --no-save sharp && node scripts/generate-icons.mjs
//
// Outputs (all committed):
//   assets/icon.png                  1024² full-bleed launcher / iOS icon
//   assets/adaptive-icon.png         1024² Android adaptive foreground (safe-zone scaled)
//   assets/adaptive-monochrome.png   1024² Android 13+ themed-icon layer
//   assets/splash-icon.png           1024² splash mark (transparent)
//   assets/notification-icon.png     96² white-on-transparent status bar icon
//   assets/favicon.png               48² web favicon
//   store/play-icon-512.png          512² Play Store listing icon
//   store/feature-graphic.png        1024×500 Play Store feature graphic
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import sharp from 'sharp';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

export const BG_TOP = '#1B2440';
export const BG_BOTTOM = '#0E1426';
const ARC_START = '#FFC24B';
const ARC_END = '#FF6B57';

// "Day progress" ring with a gap at the upper left, and a bold check.
// Drawn on a 1024 canvas centred on (512, 512).
function mark({ mono = false, scale = 1 } = {}) {
  const arcStroke = mono ? '#FFFFFF' : 'url(#arc)';
  return `
    <g transform="translate(512 512) scale(${scale}) translate(-512 -512)">
      <path d="M 512 212 A 300 300 0 1 1 230.1 409.4"
            fill="none" stroke="${arcStroke}" stroke-width="72" stroke-linecap="round"/>
      <path d="M 372 522 L 470 618 L 664 412"
            fill="none" stroke="#FFFFFF" stroke-width="84" stroke-linecap="round" stroke-linejoin="round"/>
    </g>`;
}

const defs = `
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="${BG_TOP}"/>
      <stop offset="1" stop-color="${BG_BOTTOM}"/>
    </linearGradient>
    <linearGradient id="arc" x1="0.2" y1="0" x2="0.8" y2="1">
      <stop offset="0" stop-color="${ARC_START}"/>
      <stop offset="1" stop-color="${ARC_END}"/>
    </linearGradient>
  </defs>`;

const svg = (w, h, body) =>
  `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${defs}${body}</svg>`;

const icon = svg(1024, 1024, `<rect width="1024" height="1024" fill="url(#bg)"/>${mark()}`);
// Adaptive icons are cropped to a ~66% circle; keep the mark inside it.
const adaptive = svg(1024, 1024, mark({ scale: 0.86 }));
const monochrome = svg(1024, 1024, mark({ mono: true, scale: 0.86 }));
const splash = svg(1024, 1024, mark({ scale: 0.9 }));
const notification = svg(1024, 1024, mark({ mono: true, scale: 1.25 }));

const feature = svg(1024, 500, `
  <rect width="1024" height="500" fill="url(#bg)"/>
  <g transform="translate(70 70) scale(0.35)">${mark()}</g>
  <text x="440" y="235" font-family="Helvetica Neue, Arial, sans-serif" font-size="80" font-weight="700" fill="#FFFFFF">Daily Tasks</text>
  <text x="442" y="300" font-family="Helvetica Neue, Arial, sans-serif" font-size="32" fill="#C9D2EA">Your routine, reset every morning.</text>`);

async function render(source, out, width, height = width) {
  const target = resolve(root, out);
  mkdirSync(dirname(target), { recursive: true });
  await sharp(Buffer.from(source), { density: 72 * (width / 1024) * 4 })
    .resize(width, height)
    .png({ compressionLevel: 9 })
    .toFile(target);
  console.log(`wrote ${out}`);
}

writeFileSync(resolve(root, 'assets/icon-source.svg'), icon);
await render(icon, 'assets/icon.png', 1024);
await render(adaptive, 'assets/adaptive-icon.png', 1024);
await render(monochrome, 'assets/adaptive-monochrome.png', 1024);
await render(splash, 'assets/splash-icon.png', 1024);
await render(notification, 'assets/notification-icon.png', 96);
await render(icon, 'assets/favicon.png', 48);
await render(icon, 'store/play-icon-512.png', 512);
await render(feature, 'store/feature-graphic.png', 1024, 500);
