const dropZone = document.getElementById("drop-zone");
const dropLabel = document.getElementById("drop-label");
const fileInput = document.getElementById("file-input");
const fileMeta = document.getElementById("file-meta");
const ftpInput = document.getElementById("ftp-input");
const weightInput = document.getElementById("weight-input");
const analyzeBtn = document.getElementById("analyze-btn");
const downloadBtn = document.getElementById("download-btn");
const copyBtn = document.getElementById("copy-btn");
const statusEl = document.getElementById("status");
const summaryOutput = document.getElementById("summary-output");

const worker = new Worker("./worker.js");
let selectedFile = null;
let zipBlob = null;
let zipName = "fit-analysis.zip";
let requestCounter = 0;

function setStatus(message) {
  statusEl.textContent = message;
}

function setSummary(value) {
  summaryOutput.value = value || "";
  copyBtn.disabled = !summaryOutput.value;
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
  setSummary("");

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

async function runAnalysis() {
  if (!selectedFile) {
    return;
  }

  const ftp = Number(ftpInput.value);
  const weight = Number(weightInput.value);
  if (!(ftp > 0) || !(weight > 0)) {
    setStatus("FTP and weight are required.");
    return;
  }

  setStatus("Reading file...");
  setSummary("");

  analyzeBtn.disabled = true;
  downloadBtn.disabled = true;
  copyBtn.disabled = true;

  try {
    const buffer = await selectedFile.arrayBuffer();
    const id = ++requestCounter;

    setStatus("Analyzing in browser (WASM)...");

    worker.postMessage({
      id,
      action: "analyze",
      fileBuffer: buffer,
      options: {
        source_file_name: selectedFile.name,
        ftp_w: ftp,
        weight_kg: weight,
        format: "csv",
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
      setStatus(`Analysis failed: ${response.error || "Unknown error"}`);
      return;
    }

    zipBlob = new Blob([response.zipBuffer], { type: "application/zip" });
    const stem = selectedFile.name.replace(/\.fit$/i, "");
    zipName = `${stem || "fit"}_analysis.zip`;

    downloadBtn.disabled = false;
    setSummary(response.summary || "");
    setStatus("Analysis complete. Markdown summary and ZIP are ready.");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Analysis failed: ${message}`);
  } finally {
    analyzeBtn.disabled = !(selectedFile && ftp > 0 && weight > 0);
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

async function copySummary() {
  if (!summaryOutput.value) {
    return;
  }
  try {
    await navigator.clipboard.writeText(summaryOutput.value);
    setStatus("Markdown summary copied.");
  } catch (_error) {
    summaryOutput.focus();
    summaryOutput.select();
    document.execCommand("copy");
    setStatus("Markdown summary copied.");
  }
}

fileInput.addEventListener("change", (event) => {
  const file = event.target.files && event.target.files[0];
  setSelectedFile(file);
});

[ftpInput, weightInput].forEach((input) => {
  input.addEventListener("input", validateInputs);
});

analyzeBtn.addEventListener("click", runAnalysis);
downloadBtn.addEventListener("click", downloadZip);
copyBtn.addEventListener("click", copySummary);

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

setSummary("");
validateInputs();
