// UTF-8 helpers that do not depend on TextEncoder/TextDecoder, which older
// Hermes builds lack. Used for the JSON documents exchanged with the server
// and by the polyfill in src/polyfills.ts.

export function utf8Encode(text: string): Uint8Array {
  const out: number[] = [];
  for (let i = 0; i < text.length; i += 1) {
    let code = text.charCodeAt(i);
    if (code >= 0xd800 && code <= 0xdbff && i + 1 < text.length) {
      const next = text.charCodeAt(i + 1);
      if (next >= 0xdc00 && next <= 0xdfff) {
        code = 0x10000 + ((code - 0xd800) << 10) + (next - 0xdc00);
        i += 1;
      }
    } else if (code >= 0xd800 && code <= 0xdfff) {
      code = 0xfffd; // lone surrogate
    }
    if (code < 0x80) {
      out.push(code);
    } else if (code < 0x800) {
      out.push(0xc0 | (code >> 6), 0x80 | (code & 0x3f));
    } else if (code < 0x10000) {
      out.push(0xe0 | (code >> 12), 0x80 | ((code >> 6) & 0x3f), 0x80 | (code & 0x3f));
    } else {
      out.push(
        0xf0 | (code >> 18),
        0x80 | ((code >> 12) & 0x3f),
        0x80 | ((code >> 6) & 0x3f),
        0x80 | (code & 0x3f),
      );
    }
  }
  return Uint8Array.from(out);
}

export function utf8Decode(bytes: Uint8Array): string {
  let out = '';
  let i = 0;
  while (i < bytes.length) {
    const b0 = bytes[i];
    let code: number;
    let extra: number;
    if (b0 < 0x80) {
      code = b0;
      extra = 0;
    } else if (b0 >= 0xc2 && b0 < 0xe0) {
      code = b0 & 0x1f;
      extra = 1;
    } else if (b0 >= 0xe0 && b0 < 0xf0) {
      code = b0 & 0x0f;
      extra = 2;
    } else if (b0 >= 0xf0 && b0 < 0xf5) {
      code = b0 & 0x07;
      extra = 3;
    } else {
      out += '�';
      i += 1;
      continue;
    }
    // Truncated or malformed continuation bytes become U+FFFD.
    // A malformed sequence (the lead byte plus however many continuation
    // bytes did match) collapses into a single replacement, like WHATWG.
    let matched = 0;
    for (let k = 1; k <= extra; k += 1) {
      const b = bytes[i + k];
      if (b === undefined || (b & 0xc0) !== 0x80) {
        break;
      }
      code = (code << 6) | (b & 0x3f);
      matched += 1;
    }
    if (matched < extra) {
      out += '�';
      i += 1 + matched;
      continue;
    }
    i += extra + 1;
    if (code > 0xffff) {
      code -= 0x10000;
      out += String.fromCharCode(0xd800 + (code >> 10), 0xdc00 + (code & 0x3ff));
    } else {
      out += String.fromCharCode(code);
    }
  }
  return out;
}
