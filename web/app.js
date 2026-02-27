const dropZone = document.getElementById("drop-zone");
const dropLabel = document.getElementById("drop-label");
const fileInput = document.getElementById("file-input");
const fileMeta = document.getElementById("file-meta");
const ftpInput = document.getElementById("ftp-input");
const weightInput = document.getElementById("weight-input");
const formatInput = document.getElementById("format-input");
const analyzeBtn = document.getElementById("analyze-btn");
const downloadBtn = document.getElementById("download-btn");
const statusEl = document.getElementById("status");
const warningsEl = document.getElementById("warnings");
const logEl = document.getElementById("log");

const worker = new Worker("./worker.js");
let selectedFile = null;
let zipBlob = null;
let zipName = "fit-analysis.zip";
let requestCounter = 0;

function appendLog(message) {
  const now = new Date();
  const ts = now.toLocaleTimeString();
  logEl.textContent += `[${ts}] ${message}\n`;
  logEl.scrollTop = logEl.scrollHeight;
}

function setStatus(message) {
  statusEl.textContent = message;
}

function setWarnings(list) {
  warningsEl.innerHTML = "";
  if (!list || list.length === 0) {
    const li = document.createElement("li");
    li.textContent = "No warnings.";
    warningsEl.appendChild(li);
    return;
  }
  for (const warning of list) {
    const li = document.createElement("li");
    li.textContent = warning;
    warningsEl.appendChild(li);
  }
}

function validateInputs() {
  const ftp = Number(ftpInput.value);
  const weight = Number(weightInput.value);
  const hasFile = !!selectedFile;
  const validFtp = Number.isFinite(ftp) && ftp > 0;
  const validWeight = Number.isFinite(weight) && weight > 0;

  analyzeBtn.disabled = !(hasFile && validFtp && validWeight);
  if (!hasFile) {
    setStatus("Idle. Select a .fit file.");
    return;
  }
  if (!validFtp || !validWeight) {
    setStatus("Enter required FTP and weight values to analyze.");
    return;
  }
  setStatus("Ready to analyze.");
}

function setSelectedFile(file) {
  selectedFile = file || null;
  zipBlob = null;
  downloadBtn.disabled = true;

  if (!selectedFile) {
    dropLabel.textContent = "Drag and drop a .fit file, or click to browse";
    fileMeta.textContent = "No file selected.";
    validateInputs();
    return;
  }

  dropLabel.textContent = selectedFile.name;
  fileMeta.textContent = `Selected: ${selectedFile.name} (${(selectedFile.size / 1024).toFixed(1)} KB)`;
  validateInputs();
}

function warningHintsForFile(file) {
  if (!file) return [];
  const warnings = [];
  if (!file.name.toLowerCase().endsWith(".fit")) {
    warnings.push("File extension is not .fit. Analysis may fail if this is not a FIT activity file.");
  }
  return warnings;
}

async function runAnalysis() {
  if (!selectedFile) {
    return;
  }

  const ftp = Number(ftpInput.value);
  const weight = Number(weightInput.value);
  if (!(ftp > 0) || !(weight > 0)) {
    setWarnings(["FTP and weight are required."]);
    return;
  }

  const preWarnings = warningHintsForFile(selectedFile);
  setWarnings(preWarnings);
  setStatus("Reading file...");
  appendLog(`Reading ${selectedFile.name}`);

  analyzeBtn.disabled = true;
  downloadBtn.disabled = true;

  try {
    const buffer = await selectedFile.arrayBuffer();
    const id = ++requestCounter;

    setStatus("Analyzing in browser (WASM)...");
    appendLog("Dispatching job to worker");

    worker.postMessage({
      id,
      action: "analyze",
      fileBuffer: buffer,
      options: {
        source_file_name: selectedFile.name,
        ftp_w: ftp,
        weight_kg: weight,
        format: formatInput.value,
      },
    }, [buffer]);

    const response = await new Promise((resolve) => {
      const onMessage = (event) => {
        if (!event.data || event.data.id !== id) {
          return;
        }
        worker.removeEventListener("message", onMessage);
        resolve(event.data);
      };
      worker.addEventListener("message", onMessage);
    });

    if (!response.ok) {
      appendLog(`Analysis failed: ${response.error}`);
      setStatus("Analysis failed.");
      setWarnings([...preWarnings, response.error || "Unknown error"]);
      return;
    }

    zipBlob = new Blob([response.zipBuffer], { type: "application/zip" });
    const stem = selectedFile.name.replace(/\.fit$/i, "");
    zipName = `${stem || "fit"}_analysis.zip`;

    downloadBtn.disabled = false;
    setStatus("Analysis complete. ZIP is ready.");
    appendLog(`Generated ${response.files.length} artifacts`);
    setWarnings([...preWarnings, ...(response.warnings || [])]);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    appendLog(`Unexpected error: ${message}`);
    setStatus("Analysis failed.");
    setWarnings([message]);
  } finally {
    validateInputs();
  }
}

function downloadZip() {
  if (!zipBlob) {
    return;
  }
  const url = URL.createObjectURL(zipBlob);
  const a = document.createElement("a");
  a.href = url;
  a.download = zipName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

fileInput.addEventListener("change", (event) => {
  const file = event.target.files && event.target.files[0];
  setSelectedFile(file);
});

[ftpInput, weightInput, formatInput].forEach((input) => {
  input.addEventListener("input", validateInputs);
});

analyzeBtn.addEventListener("click", runAnalysis);
downloadBtn.addEventListener("click", downloadZip);

dropZone.addEventListener("dragover", (event) => {
  event.preventDefault();
  dropZone.classList.add("dragging");
});

dropZone.addEventListener("dragleave", () => {
  dropZone.classList.remove("dragging");
});

dropZone.addEventListener("drop", (event) => {
  event.preventDefault();
  dropZone.classList.remove("dragging");
  const file = event.dataTransfer.files && event.dataTransfer.files[0];
  setSelectedFile(file);
});

appendLog("UI initialized");
setWarnings([]);
validateInputs();
