/* global Go */
let runtimeReady = null;

async function initRuntime() {
  if (runtimeReady) {
    return runtimeReady;
  }
  runtimeReady = (async () => {
    importScripts("./wasm_exec.js");
    const go = new Go();
    const resp = await fetch("./fit_analyzer.wasm");
    const bytes = await resp.arrayBuffer();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(result.instance);
    if (typeof self.analyzeFit !== "function") {
      throw new Error("analyzeFit function was not registered by WASM runtime");
    }
  })();
  return runtimeReady;
}

self.onmessage = async (event) => {
  const { id, action, fileBuffer, options } = event.data || {};
  if (action !== "analyze") {
    return;
  }

  try {
    await initRuntime();
    const fitBytes = new Uint8Array(fileBuffer);
    const result = self.analyzeFit(fitBytes, options || {});
    if (!result || result.ok !== true) {
      const message = (result && result.error) || "analysis failed";
      self.postMessage({ id, ok: false, error: message });
      return;
    }

    const zipBytes = result.zip;
    if (!zipBytes || typeof zipBytes.length !== "number") {
      self.postMessage({ id, ok: false, error: "WASM returned invalid zip payload" });
      return;
    }

    const zipCopy = new Uint8Array(zipBytes.length);
    zipCopy.set(zipBytes);
    self.postMessage(
      {
        id,
        ok: true,
        summary: typeof result.summary_md === "string" ? result.summary_md : "",
        warnings: Array.isArray(result.warnings) ? result.warnings : [],
        files: Array.isArray(result.files) ? result.files : [],
        zipBuffer: zipCopy.buffer,
      },
      [zipCopy.buffer],
    );
  } catch (error) {
    self.postMessage({
      id,
      ok: false,
      error: error instanceof Error ? error.message : String(error),
    });
  }
};
