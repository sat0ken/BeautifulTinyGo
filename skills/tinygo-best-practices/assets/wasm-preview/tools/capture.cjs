#!/usr/bin/env node
// Headless check for TinyGo display programs built with -target=wasm.
//
// It runs main.wasm in Node with TinyGo's wasm_exec.js, plays the same role as
// web/preview.js (previewInit, display, previewKey), presses keys on a schedule
// and writes PNG snapshots. No browser is needed, so it works in CI and in
// sandboxes. Look at the PNGs afterwards; the summary line also flags a blank
// screen.
//
// usage:
//   node tools/capture.cjs web/main.wasm [options]
//
// options:
//   --exec FILE      wasm_exec.js to use (default: web/wasm_exec.js next to the
//                    wasm file, else $(tinygo env TINYGOROOT)/targets/wasm_exec.js)
//   --out DIR        where to write PNGs (default: preview-out)
//   --duration MS    how long to run the program (default: 2000)
//   --at MS,MS,...   when to take snapshots (default: the end of --duration)
//   --keys SPEC      scripted input, comma separated "start:key:hold" in ms,
//                    e.g. "300:ArrowRight:100,800:z:50". Use "Space" for " ".
//   --scale N        nearest-neighbour upscale of the PNGs (default: 4)
//   --size WxH       frame size for programs that write window.screenBuffer
//                    without calling previewInit, such as koebiten (default: 128x64)
//
// It also drives koebiten's wasm build: koebiten reads window.screenBuffer and
// calls window.wasmKeyEvent(key) on every frame while a key is held.
"use strict";

const fs = require("fs");
const path = require("path");
const zlib = require("zlib");
const { execSync } = require("child_process");

function parseArgs(argv) {
  const opts = { duration: 2000, out: "preview-out", scale: 4, at: null, keys: [], size: [128, 64], exec: null };
  const rest = [];
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => {
      if (i + 1 >= argv.length) throw new Error(`${a} needs a value`);
      return argv[++i];
    };
    switch (a) {
      case "--exec": opts.exec = next(); break;
      case "--out": opts.out = next(); break;
      case "--duration": opts.duration = Number(next()); break;
      case "--at": opts.at = next().split(",").map(Number); break;
      case "--scale": opts.scale = Math.max(1, Number(next()) | 0); break;
      case "--size": opts.size = next().split("x").map(Number); break;
      case "--keys":
        opts.keys = next().split(",").filter(Boolean).map((s) => {
          const [start, key, hold] = s.split(":");
          return { start: Number(start), key: key === "Space" ? " " : key, hold: Number(hold || 100) };
        });
        break;
      default:
        if (a.startsWith("--")) throw new Error(`unknown option ${a}`);
        rest.push(a);
    }
  }
  if (rest.length !== 1) throw new Error("usage: node tools/capture.cjs path/to/main.wasm [options]");
  opts.wasm = rest[0];
  if (!opts.at) opts.at = [opts.duration];
  return opts;
}

function findWasmExec(opts) {
  if (opts.exec) return path.resolve(opts.exec);
  const local = path.join(path.dirname(path.resolve(opts.wasm)), "wasm_exec.js");
  if (fs.existsSync(local)) return local;
  const root = execSync("tinygo env TINYGOROOT", { encoding: "utf8" }).trim();
  return path.join(root, "targets", "wasm_exec.js");
}

// Minimal PNG writer: 8-bit RGBA, filter type 0 on every row.
const crcTable = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();
function crc32(buf) {
  let c = 0xffffffff;
  for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const td = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(td));
  return Buffer.concat([len, td, crc]);
}
function encodePNG(rgba, w, h, scale) {
  const W = w * scale, H = h * scale;
  const raw = Buffer.alloc((W * 4 + 1) * H);
  for (let y = 0; y < H; y++) {
    const row = y * (W * 4 + 1);
    raw[row] = 0;
    const sy = Math.floor(y / scale);
    for (let x = 0; x < W; x++) {
      const s = (sy * w + Math.floor(x / scale)) * 4;
      const d = row + 1 + x * 4;
      raw[d] = rgba[s];
      raw[d + 1] = rgba[s + 1];
      raw[d + 2] = rgba[s + 2];
      raw[d + 3] = 255; // the preview is always opaque
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(W, 0);
  ihdr.writeUInt32BE(H, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 6; // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", zlib.deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

function colorCount(rgba) {
  const seen = new Set();
  for (let i = 0; i < rgba.length && seen.size < 256; i += 4) {
    seen.add((rgba[i] << 16) | (rgba[i + 1] << 8) | rgba[i + 2]);
  }
  return seen.size;
}

function main() {
  const opts = parseArgs(process.argv.slice(2));

  // Same globals as TinyGo's targets/wasm_exec_node.js.
  globalThis.require = require;
  globalThis.fs = fs;
  require(findWasmExec(opts));

  let width = opts.size[0], height = opts.size[1];
  globalThis.screenBuffer = new Uint8Array(width * height * 4);
  globalThis.previewInit = (w, h) => {
    width = w;
    height = h;
    globalThis.screenBuffer = new Uint8Array(w * h * 4);
    return globalThis.screenBuffer;
  };
  let frames = 0;
  let latest = null;
  globalThis.display = () => {
    frames++;
    latest = Uint8Array.from(globalThis.screenBuffer);
  };

  // Scripted key presses.
  const held = new Set();
  for (const k of opts.keys) {
    setTimeout(() => {
      held.add(k.key);
      if (typeof globalThis.previewKey === "function") globalThis.previewKey(k.key, true);
    }, k.start);
    setTimeout(() => {
      held.delete(k.key);
      if (typeof globalThis.previewKey === "function") globalThis.previewKey(k.key, false);
    }, k.start + k.hold);
  }
  const repeat = setInterval(() => {
    if (typeof globalThis.wasmKeyEvent !== "function") return;
    for (const key of held) globalThis.wasmKeyEvent(key);
  }, 16);

  fs.mkdirSync(opts.out, { recursive: true });
  const written = [];
  const snap = (ms) => {
    if (!latest) {
      console.error(`[capture] ${ms}ms: no frame yet (display() was never called)`);
      return;
    }
    const file = path.join(opts.out, `frame-${String(ms).padStart(5, "0")}ms.png`);
    fs.writeFileSync(file, encodePNG(latest, width, height, opts.scale));
    written.push({ ms, file, colors: colorCount(latest) });
  };
  for (const ms of opts.at) setTimeout(() => snap(ms), ms);

  const finish = (code) => {
    clearInterval(repeat);
    const blank = written.length > 0 && written.every((w) => w.colors <= 1);
    console.log(JSON.stringify({ size: `${width}x${height}`, frames, snapshots: written, blank }));
    if (blank) console.error("[capture] every snapshot is a single solid color: the program drew nothing visible");
    process.exit(code);
  };
  setTimeout(() => finish(frames > 0 ? 0 : 2), Math.max(opts.duration, ...opts.at) + 20);

  const go = new Go();
  WebAssembly.instantiate(fs.readFileSync(opts.wasm), go.importObject)
    .then((r) => go.run(r.instance))
    .then(() => {
      // The program returned from main. Keep the snapshots that were taken.
      for (const ms of opts.at) if (!written.some((w) => w.ms === ms)) snap(ms);
      finish(0);
    })
    .catch((err) => {
      console.error("[capture] the program failed:", err);
      finish(1);
    });
}

try {
  main();
} catch (err) {
  console.error(String(err.message || err));
  process.exit(64);
}
