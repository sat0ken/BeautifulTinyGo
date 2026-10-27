// Browser side of the TinyGo display preview.
//
// Contract with the Go side (package preview):
//   previewInit(width, height) -> Uint8Array   called once by preview.NewDisplay
//   display()                                  called after each frame copy
//   previewKey(key, down)                      defined by Go, called here on key events
// tools/capture.cjs implements the same contract in Node for headless checks.
"use strict";

(() => {
  const canvas = document.getElementById("screen");
  const status = document.getElementById("status");
  const ctx = canvas.getContext("2d");
  let image = null; // ImageData at native resolution
  let off = null;   // offscreen canvas at native resolution
  let frames = 0, last = performance.now(), fps = 0;

  window.previewInit = (w, h) => {
    off = document.createElement("canvas");
    off.width = w;
    off.height = h;
    image = off.getContext("2d").createImageData(w, h);
    // Integer scale that fits the window, so pixels stay square and sharp.
    const scale = Math.max(1, Math.min(8, Math.floor(Math.min((innerWidth - 60) / w, (innerHeight - 160) / h))));
    canvas.width = w * scale;
    canvas.height = h * scale;
    ctx.imageSmoothingEnabled = false;
    window.screenBuffer = new Uint8Array(w * h * 4);
    return window.screenBuffer;
  };

  window.display = () => {
    if (!image) return;
    image.data.set(window.screenBuffer);
    off.getContext("2d").putImageData(image, 0, 0);
    ctx.drawImage(off, 0, 0, canvas.width, canvas.height);
    frames++;
    const now = performance.now();
    if (now - last >= 1000) {
      fps = frames * 1000 / (now - last);
      frames = 0;
      last = now;
      status.textContent = `${off.width}×${off.height}  ${fps.toFixed(1)} fps`;
    }
  };

  const send = (e, down) => {
    if (typeof window.previewKey !== "function") return;
    if (e.key.startsWith("Arrow") || e.key === " ") e.preventDefault();
    if (e.repeat) return;
    window.previewKey(e.key, down);
  };
  addEventListener("keydown", (e) => send(e, true));
  addEventListener("keyup", (e) => send(e, false));

  // fetch + instantiate works even when the server sends .wasm with the wrong
  // MIME type, which breaks instantiateStreaming on some static servers.
  const go = new Go();
  fetch("main.wasm")
    .then((r) => {
      if (!r.ok) throw new Error(`main.wasm: HTTP ${r.status}`);
      return r.arrayBuffer();
    })
    .then((bytes) => WebAssembly.instantiate(bytes, go.importObject))
    .then((obj) => {
      status.textContent = "running";
      return go.run(obj.instance);
    })
    .then(() => { status.textContent = "program exited"; })
    .catch((err) => {
      status.textContent = String(err);
      console.error(err);
    });
})();
