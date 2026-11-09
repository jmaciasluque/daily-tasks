// Must be imported before anything that uses protobuf-es / Connect.
// Hermes ships TextEncoder; TextDecoder arrived later. Install minimal UTF-8
// implementations only where missing.
import { utf8Decode, utf8Encode } from './services/utf8';

const g = globalThis as unknown as Record<string, unknown>;

class Utf8TextEncoder {
  readonly encoding = 'utf-8';

  encode(input = ''): Uint8Array {
    return utf8Encode(input);
  }

  encodeInto(input: string, dest: Uint8Array): { read: number; written: number } {
    const bytes = utf8Encode(input);
    const written = Math.min(bytes.length, dest.length);
    dest.set(bytes.subarray(0, written));
    return { read: input.length, written };
  }
}

class Utf8TextDecoder {
  readonly encoding = 'utf-8';
  readonly fatal = false;
  readonly ignoreBOM = false;

  decode(input?: ArrayBuffer | ArrayBufferView): string {
    if (!input) {
      return '';
    }
    const bytes = input instanceof ArrayBuffer
      ? new Uint8Array(input)
      : new Uint8Array(input.buffer, input.byteOffset, input.byteLength);
    return utf8Decode(bytes);
  }
}

if (typeof g.TextEncoder === 'undefined') {
  g.TextEncoder = Utf8TextEncoder;
}
if (typeof g.TextDecoder === 'undefined') {
  g.TextDecoder = Utf8TextDecoder;
}
