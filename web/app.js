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
const resultsShell = document.getElementById("results-shell");
const sessionTitle = document.getElementById("session-title");
const sessionMeta = document.getElementById("session-meta");
const metricGrid = document.getElementById("metric-grid");
const structureOutput = document.getElementById("structure-output");
const intervalOutput = document.getElementById("interval-output");
const notesOutput = document.getElementById("notes-output");
const warningsList = document.getElementById("warnings-list");
const artifactList = document.getElementById("artifact-list");
const summaryOutput = document.getElementById("summary-output");

const worker = new Worker("./worker.js");
let selectedFile = null;
let zipBlob = null;
let zipName = "fit-analysis.zip";
let requestCounter = 0;

function positiveNumber(input) {
  const value = Number(input.value);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function setStatus(message, tone = "") {
  statusEl.textContent = message;
  statusEl.dataset.tone = tone;
}

function setSummary(value) {
  summaryOutput.value = value || "";
  copyBtn.disabled = !summaryOutput.value;
}

function resetResults() {
  renderAnalysis(null);
  renderWarnings([]);
  renderArtifacts([]);
  setSummary("");
}

function validateInputs() {
  const hasFile = !!selectedFile;
  analyzeBtn.disabled = !hasFile;

  if (!hasFile) {
    setStatus("Idle. Select a .fit file.");
    return;
  }

  const ftp = positiveNumber(ftpInput);
  const weight = positiveNumber(weightInput);
  const parts = [];
  if (ftp > 0) {
    parts.push(`FTP ${Math.round(ftp)} W`);
  }
  if (weight > 0) {
    parts.push(`weight ${weight.toFixed(1)} kg`);
  }
  setStatus(parts.length > 0 ? `Ready to analyze with ${parts.join(" and ")}.` : "Ready to analyze. FTP and weight are optional.");
}

function setSelectedFile(file) {
  selectedFile = file || null;
  zipBlob = null;
  downloadBtn.disabled = true;
  resetResults();

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

function cleanLabel(value) {
  const text = String(value || "").trim();
  if (!text) {
    return "Unknown";
  }
  return text
    .replace(/_/g, " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function formatDuration(seconds) {
  if (!(seconds > 0)) {
    return "0s";
  }
  const total = Math.round(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) {
    return `${hours}h ${String(minutes).padStart(2, "0")}m`;
  }
  if (minutes > 0) {
    return `${minutes}m ${String(secs).padStart(2, "0")}s`;
  }
  return `${secs}s`;
}

function formatDateTime(value) {
  if (!value) {
    return "Unknown start";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function formatDistance(meters) {
  return meters > 0 ? `${(meters / 1000).toFixed(1)} km` : "0.0 km";
}

function formatElevation(meters) {
  return meters > 0 ? `+${Math.round(meters)} m` : "0 m";
}

function formatWatts(value) {
  return value > 0 ? `${Math.round(value)} W` : "--";
}

function formatBPM(value) {
  return value > 0 ? `${Math.round(value)} bpm` : "--";
}

function formatRPM(value) {
  return value > 0 ? `${Math.round(value)} rpm` : "--";
}

function formatSpeed(mps) {
  return mps > 0 ? `${(mps * 3.6).toFixed(1)} km/h` : "--";
}

function formatPercent(value, digits = 0) {
  return Number.isFinite(value) && value !== 0 ? `${value.toFixed(digits)}%` : "--";
}

function createMetricCard(value, label) {
  const card = document.createElement("article");
  card.className = "metric-card";

  const strong = document.createElement("strong");
  strong.textContent = value;
  card.appendChild(strong);

  const span = document.createElement("span");
  span.className = "metric-label";
  span.textContent = label;
  card.appendChild(span);

  return card;
}

function renderMetricGrid(analysis) {
  metricGrid.replaceChildren();
  if (!analysis) {
    [
      createMetricCard("--", "Duration"),
      createMetricCard("--", "Distance"),
      createMetricCard("--", "Power"),
      createMetricCard("--", "Load"),
    ].forEach((card) => metricGrid.appendChild(card));
    return;
  }

  const loadValue = analysis.ftp_watts > 0
    ? `IF ${analysis.intensity_factor.toFixed(2)} · TSS ${Math.round(analysis.training_stress_score || 0)}`
    : analysis.ftp_source === "estimated"
      ? `FTP ${Math.round(analysis.ftp_watts || 0)} W est.`
      : "FTP unavailable";

  const cards = [
    [formatDuration(analysis.elapsed_seconds), "Duration"],
    [formatDistance(analysis.distance_meters), "Distance"],
    [formatElevation(analysis.elevation_gain_m), "Elevation Gain"],
    [`${formatWatts(analysis.avg_power_watts)} avg`, "Average Power"],
    [`${formatWatts(analysis.normalized_power_watts)} NP`, "Normalized Power"],
    [loadValue, "Load"],
    [`${formatBPM(analysis.avg_heart_rate_bpm)} avg`, "Heart Rate"],
    [`${formatRPM(analysis.avg_cadence_rpm)} avg`, "Cadence"],
    [`${formatSpeed(analysis.avg_speed_mps)} avg`, "Speed"],
    [`${Math.round(analysis.work_kilojoules || 0)} kJ`, "Work"],
  ];

  if (analysis.weight_kg > 0 && analysis.np_w_per_kg > 0) {
    cards.push([`${analysis.np_w_per_kg.toFixed(2)} W/kg`, "NP W/kg"]);
  }
  if (analysis.best_20min_power_watts > 0) {
    cards.push([formatWatts(analysis.best_20min_power_watts), "Best 20 Min"]);
  }

  cards.forEach(([value, label]) => metricGrid.appendChild(createMetricCard(value, label)));
}

function renderAnalysis(analysis) {
  resultsShell.classList.toggle("is-ready", !!analysis);
  renderMetricGrid(analysis);

  if (!analysis) {
    sessionTitle.textContent = "No analysis yet";
    sessionMeta.textContent = "Analyze a file to populate the shared analyzer output.";
    structureOutput.className = "stack empty-state";
    structureOutput.textContent = "Analyze a FIT file to map the workout blocks.";
    intervalOutput.className = "stack empty-state";
    intervalOutput.textContent = "Interval execution stats appear here.";
    notesOutput.className = "notes-output empty-state";
    notesOutput.textContent = "The analyzer notes will appear here.";
    return;
  }

  const titleParts = [cleanLabel(analysis.sport)];
  if (analysis.sub_sport && analysis.sub_sport !== "Generic") {
    titleParts.push(cleanLabel(analysis.sub_sport));
  }
  sessionTitle.textContent = titleParts.join(" / ");

  const metaParts = [formatDateTime(analysis.start_time)];
  if (analysis.ftp_watts > 0) {
    metaParts.push(`FTP ${Math.round(analysis.ftp_watts)} W (${analysis.ftp_source || "input"})`);
  }
  if (analysis.weight_kg > 0) {
    metaParts.push(`${analysis.weight_kg.toFixed(1)} kg`);
  }
  sessionMeta.textContent = metaParts.join(" • ");

  renderStructure(analysis.workout_structure);
  renderIntervals(analysis.intervals, analysis.workout_structure);
  notesOutput.className = "notes-output";
  notesOutput.textContent = analysis.notes || "No notes available.";
}

function renderStructure(workoutStructure) {
  structureOutput.replaceChildren();
  structureOutput.className = "stack";

  if (!workoutStructure || !Array.isArray(workoutStructure.blocks) || workoutStructure.blocks.length === 0) {
    structureOutput.classList.add("empty-state");
    structureOutput.textContent = "Workout structure could not be inferred from the available lap data.";
    return;
  }

  const summary = document.createElement("article");
  summary.className = "structure-summary";
  const summaryTitle = document.createElement("strong");
  summaryTitle.textContent = workoutStructure.canonical_label || "Structured session";
  summary.appendChild(summaryTitle);
  const summaryMeta = document.createElement("div");
  summaryMeta.className = "block-meta";
  summaryMeta.textContent = `Confidence ${Math.round((workoutStructure.confidence || 0) * 100)}%`;
  summary.appendChild(summaryMeta);
  structureOutput.appendChild(summary);

  workoutStructure.blocks.forEach((block) => {
    const card = document.createElement("article");
    card.className = "structure-block";

    const header = document.createElement("div");
    header.className = "structure-block-header";

    const title = document.createElement("strong");
    title.textContent = cleanLabel(block.block_type);
    header.appendChild(title);

    const pill = document.createElement("span");
    pill.className = "pill";
    pill.textContent = formatDuration(block.duration_seconds);
    header.appendChild(pill);
    card.appendChild(header);

    const meta = document.createElement("div");
    meta.className = "block-meta";
    const details = [];
    if (block.avg_power_watts > 0) {
      details.push(`${Math.round(block.avg_power_watts)} W avg`);
    }
    if (block.avg_heart_rate_bpm > 0) {
      details.push(`${Math.round(block.avg_heart_rate_bpm)} bpm avg`);
    }
    if (block.avg_cadence_rpm > 0) {
      details.push(`${Math.round(block.avg_cadence_rpm)} rpm avg`);
    }
    meta.textContent = details.join(" • ");
    if (details.length > 0) {
      card.appendChild(meta);
    }

    const desc = document.createElement("div");
    desc.className = "block-meta";
    desc.textContent = block.description || "No block description available.";
    card.appendChild(desc);

    structureOutput.appendChild(card);
  });
}

function createReadoutCard(value, label) {
  const card = document.createElement("article");
  card.className = "readout-card";

  const strong = document.createElement("strong");
  strong.textContent = value;
  card.appendChild(strong);

  const span = document.createElement("span");
  span.className = "metric-label";
  span.textContent = label;
  card.appendChild(span);

  return card;
}

function renderIntervals(intervals, workoutStructure) {
  intervalOutput.replaceChildren();
  intervalOutput.className = "stack";

  if (!intervals || !(intervals.work_count > 0)) {
    intervalOutput.classList.add("empty-state");
    intervalOutput.textContent = "No repeating work intervals were confidently detected from the lap data.";
    return;
  }

  if (workoutStructure && workoutStructure.main_set && workoutStructure.main_set.prescription) {
    const summary = document.createElement("article");
    summary.className = "structure-summary";
    const strong = document.createElement("strong");
    strong.textContent = workoutStructure.main_set.prescription;
    summary.appendChild(strong);
    const meta = document.createElement("div");
    meta.className = "block-meta";
    meta.textContent = `${intervals.work_count} work reps • ${intervals.recovery_count} recoveries • ${intervals.activation_count || 0} activations`;
    summary.appendChild(meta);
    intervalOutput.appendChild(summary);
  }

  const grid = document.createElement("div");
  grid.className = "readout-grid";
  [
    createReadoutCard(formatWatts(intervals.avg_work_power_watts), "Work Avg"),
    createReadoutCard(formatDuration(intervals.avg_work_duration_seconds), "Work Duration"),
    createReadoutCard(intervals.recovery_count > 0 ? formatWatts(intervals.avg_recovery_power_watts) : "--", "Recovery Avg"),
    createReadoutCard(intervals.recovery_count > 0 ? formatDuration(intervals.avg_recovery_duration_seconds) : "--", "Recovery Duration"),
    createReadoutCard(formatPercent(intervals.work_power_change_pct, 1), "Power Drift"),
    createReadoutCard(intervals.work_heart_rate_change_bpm ? `${Math.round(intervals.work_heart_rate_change_bpm)} bpm` : "--", "HR Drift"),
  ].forEach((card) => grid.appendChild(card));

  intervalOutput.appendChild(grid);
}

function renderWarnings(warnings) {
  warningsList.replaceChildren();
  warningsList.className = "list";

  if (!warnings || warnings.length === 0) {
    warningsList.classList.add("muted-list");
    const item = document.createElement("li");
    item.textContent = "No warnings.";
    warningsList.appendChild(item);
    return;
  }

  warnings.forEach((warning) => {
    const item = document.createElement("li");
    item.textContent = warning;
    warningsList.appendChild(item);
  });
}

function renderArtifacts(files) {
  artifactList.replaceChildren();
  artifactList.className = "list";

  if (!files || files.length === 0) {
    artifactList.classList.add("muted-list");
    const item = document.createElement("li");
    item.textContent = "Run an analysis to build the artifact bundle.";
    artifactList.appendChild(item);
    return;
  }

  files.forEach((name) => {
    const item = document.createElement("li");
    item.textContent = name;
    artifactList.appendChild(item);
  });
}

async function runAnalysis() {
  if (!selectedFile) {
    return;
  }

  setStatus("Reading file...");
  resetResults();
  analyzeBtn.disabled = true;
  downloadBtn.disabled = true;
  copyBtn.disabled = true;

  try {
    const buffer = await selectedFile.arrayBuffer();
    const id = ++requestCounter;

    setStatus("Analyzing in browser with the shared Go library...");
    worker.postMessage(
      {
        id,
        action: "analyze",
        fileBuffer: buffer,
        options: {
          source_file_name: selectedFile.name,
          ftp_w: positiveNumber(ftpInput),
          weight_kg: positiveNumber(weightInput),
          format: "csv",
        },
      },
      [buffer],
    );

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
      setStatus(`Analysis failed: ${response.error || "Unknown error"}`, "error");
      return;
    }

    zipBlob = new Blob([response.zipBuffer], { type: "application/zip" });
    const stem = selectedFile.name.replace(/\.fit$/i, "");
    zipName = `${stem || "fit"}_analysis.zip`;
    downloadBtn.disabled = false;

    const analysis = response.analysisJSON ? JSON.parse(response.analysisJSON) : null;
    renderAnalysis(analysis);
    renderWarnings(Array.isArray(response.warnings) ? response.warnings : []);
    renderArtifacts(Array.isArray(response.files) ? response.files : []);
    setSummary(response.summary || "");

    const warningCount = Array.isArray(response.warnings) ? response.warnings.length : 0;
    setStatus(
      warningCount > 0
        ? `Analysis complete with ${warningCount} warning${warningCount === 1 ? "" : "s"}.`
        : "Analysis complete. Summary and artifact bundle are ready.",
      "success",
    );
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Analysis failed: ${message}`, "error");
  } finally {
    analyzeBtn.disabled = !selectedFile;
  }
}

function downloadZip() {
  if (!zipBlob) {
    return;
  }
  const url = URL.createObjectURL(zipBlob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = zipName;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

async function copySummary() {
  if (!summaryOutput.value) {
    return;
  }
  try {
    await navigator.clipboard.writeText(summaryOutput.value);
  } catch (_error) {
    summaryOutput.focus();
    summaryOutput.select();
    document.execCommand("copy");
  }
  setStatus("Markdown summary copied.", "success");
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

resetResults();
validateInputs();
