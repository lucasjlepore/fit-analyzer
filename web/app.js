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

const courseDropZone = document.getElementById("course-drop-zone");
const courseDropLabel = document.getElementById("course-drop-label");
const courseFileInput = document.getElementById("course-file-input");
const courseFileMeta = document.getElementById("course-file-meta");
const profileImportInput = document.getElementById("profile-import-input");
const profileImportMeta = document.getElementById("profile-import-meta");
const raceFTPInput = document.getElementById("race-ftp-input");
const raceWeightInput = document.getElementById("race-weight-input");
const carbInput = document.getElementById("carb-input");
const bottleInput = document.getElementById("bottle-input");
const bottlesInput = document.getElementById("bottles-input");
const caffeineInput = document.getElementById("caffeine-input");
const goalInput = document.getElementById("goal-input");
const riderTypeInput = document.getElementById("rider-type-input");
const weeklyHoursInput = document.getElementById("weekly-hours-input");
const weeklyKMInput = document.getElementById("weekly-km-input");
const longestRideInput = document.getElementById("longest-ride-input");
const teamSupportInput = document.getElementById("team-support-input");
const technicalInput = document.getElementById("technical-input");
const strategyInput = document.getElementById("strategy-input");
const racePlanBtn = document.getElementById("race-plan-btn");
const raceDownloadBtn = document.getElementById("race-download-btn");
const raceCopyBtn = document.getElementById("race-copy-btn");
const raceStatusEl = document.getElementById("race-status");
const raceResultsShell = document.getElementById("race-results-shell");
const courseTitle = document.getElementById("course-title");
const courseMeta = document.getElementById("course-meta");
const raceMetricGrid = document.getElementById("race-metric-grid");
const climbOutput = document.getElementById("climb-output");
const pressureOutput = document.getElementById("pressure-output");
const fuelOutput = document.getElementById("fuel-output");
const sectorOutput = document.getElementById("sector-output");
const raceWarningsList = document.getElementById("race-warnings-list");
const raceArtifactList = document.getElementById("race-artifact-list");
const raceSummaryOutput = document.getElementById("race-summary-output");

const worker = new Worker("./worker.js");
let selectedFile = null;
let zipBlob = null;
let zipName = "fit-analysis.zip";

let selectedCourseFile = null;
let raceZipBlob = null;
let raceZipName = "race-plan.zip";
let importedProfileWarning = "";

let requestCounter = 0;

function positiveNumber(input) {
  const value = Number(input.value);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function nonNegativeInteger(input) {
  const value = Number(input.value);
  return Number.isFinite(value) && value >= 0 ? Math.round(value) : 0;
}

function optionalNumber(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function setNumberInput(input, value, digits = 0) {
  if (!(value > 0)) {
    return;
  }
  input.value = digits > 0 ? Number(value).toFixed(digits) : String(Math.round(value));
}

function setSelectInput(input, value, allowed) {
  const normalized = String(value || "").trim();
  if (!normalized || !allowed.includes(normalized)) {
    return;
  }
  input.value = normalized;
}

function extractImportedProfile(payload) {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const profile = payload.profile && typeof payload.profile === "object" ? payload.profile : payload;
  const training = payload.derived_training && typeof payload.derived_training === "object" ? payload.derived_training : {};
  const hasStartBottles = Object.prototype.hasOwnProperty.call(profile, "start_bottles");
  return {
    athleteName: payload.athlete && typeof payload.athlete === "object" ? String(payload.athlete.display_name || payload.athlete.id || "").trim() : "",
    weeklyHours: Number(profile.weekly_hours || training.weekly_hours || 0),
    weeklyKM: Number(profile.weekly_km || training.weekly_km || 0),
    longestRideKM: Number(profile.longest_recent_ride_km || training.longest_recent_ride_km || 0),
    ftp: Number(profile.ftp_w || profile.ftp_watts || 0),
    weight: Number(profile.weight_kg || 0),
    carbs: Number(profile.max_carb_g_per_h || profile.max_carb_g_per_hour || 0),
    bottleML: Number(profile.bottle_ml || 0),
    startBottles: hasStartBottles ? optionalNumber(profile.start_bottles) : null,
    caffeine: Number(profile.caffeine_mg_per_kg || 0),
    goal: String(profile.goal || "").trim(),
    riderType: String(profile.rider_type || "").trim(),
    teamSupport: String(profile.team_support || "").trim(),
    technical: String(profile.technical_confidence || "").trim(),
    strategyMode: String(profile.strategy_mode || "").trim(),
    warnings: Array.isArray(payload.warnings) ? payload.warnings.filter((value) => typeof value === "string") : [],
  };
}

function setStatus(element, message, tone = "") {
  element.textContent = message;
  element.dataset.tone = tone;
}

function setSummary(textarea, button, value) {
  textarea.value = value || "";
  button.disabled = !textarea.value;
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

function createInfoCard(title, meta, body, pillText = "") {
  const card = document.createElement("article");
  card.className = "structure-block";

  const header = document.createElement("div");
  header.className = "structure-block-header";

  const strong = document.createElement("strong");
  strong.textContent = title;
  header.appendChild(strong);

  if (pillText) {
    const pill = document.createElement("span");
    pill.className = "pill";
    pill.textContent = pillText;
    header.appendChild(pill);
  }
  card.appendChild(header);

  if (meta) {
    const metaEl = document.createElement("div");
    metaEl.className = "block-meta";
    metaEl.textContent = meta;
    card.appendChild(metaEl);
  }

  if (body) {
    const bodyEl = document.createElement("div");
    bodyEl.className = "block-meta";
    bodyEl.textContent = body;
    card.appendChild(bodyEl);
  }

  return card;
}

function renderStringList(listEl, items, emptyMessage) {
  listEl.replaceChildren();
  listEl.className = "list";

  if (!items || items.length === 0) {
    listEl.classList.add("muted-list");
    const item = document.createElement("li");
    item.textContent = emptyMessage;
    listEl.appendChild(item);
    return;
  }

  items.forEach((value) => {
    const item = document.createElement("li");
    item.textContent = value;
    listEl.appendChild(item);
  });
}

function resetStack(container, message) {
  container.replaceChildren();
  container.className = "stack empty-state";
  container.textContent = message;
}

function renderArtifacts(listEl, files, emptyMessage) {
  renderStringList(listEl, files, emptyMessage);
}

function renderWarnings(listEl, warnings) {
  renderStringList(listEl, warnings, "No warnings.");
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
    structureOutput.appendChild(
      createInfoCard(
        cleanLabel(block.block_type),
        details.join(" • "),
        block.description || "No block description available.",
        formatDuration(block.duration_seconds),
      ),
    );
  });
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

function renderAnalysis(analysis) {
  resultsShell.classList.toggle("is-ready", !!analysis);
  renderMetricGrid(analysis);

  if (!analysis) {
    sessionTitle.textContent = "No analysis yet";
    sessionMeta.textContent = "Analyze a file to populate the shared analyzer output.";
    resetStack(structureOutput, "Analyze a FIT file to map the workout blocks.");
    resetStack(intervalOutput, "Interval execution stats appear here.");
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

function resetRideResults() {
  renderAnalysis(null);
  renderWarnings(warningsList, []);
  renderArtifacts(artifactList, [], "Run an analysis to build the artifact bundle.");
  setSummary(summaryOutput, copyBtn, "");
}

function validateRideInputs() {
  const hasFile = !!selectedFile;
  analyzeBtn.disabled = !hasFile;

  if (!hasFile) {
    setStatus(statusEl, "Idle. Select a .fit file.");
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
  setStatus(statusEl, parts.length > 0 ? `Ready to analyze with ${parts.join(" and ")}.` : "Ready to analyze. FTP and weight are optional.");
}

function setSelectedRideFile(file) {
  selectedFile = file || null;
  zipBlob = null;
  downloadBtn.disabled = true;
  resetRideResults();

  if (!selectedFile) {
    dropLabel.textContent = "Drag and drop a .fit file, or click to browse";
    fileMeta.textContent = "No file selected.";
    validateRideInputs();
    return;
  }

  dropLabel.textContent = selectedFile.name;
  fileMeta.textContent = `Selected: ${selectedFile.name} (${(selectedFile.size / 1024).toFixed(1)} KB)`;
  validateRideInputs();
}

function renderRaceMetricGrid(plan) {
  raceMetricGrid.replaceChildren();
  if (!plan) {
    [
      createMetricCard("--", "Distance"),
      createMetricCard("--", "Elevation"),
      createMetricCard("--", "Duration"),
      createMetricCard("--", "Fuel"),
    ].forEach((card) => raceMetricGrid.appendChild(card));
    return;
  }

  const cards = [
    [formatDistance(plan.distance_meters), "Distance"],
    [formatElevation(plan.elevation_gain_m), "Elevation Gain"],
    [formatDuration(plan.estimated_duration_seconds), "Est. Duration"],
    [`${(plan.estimated_average_speed_kph || 0).toFixed(1)} km/h`, "Est. Avg Speed"],
    [cleanLabel(plan.rider_type), "Rider Type"],
    [`${Math.round(plan.fuel_plan.carb_target_g_per_hour || 0)} g/h`, "Carb Target"],
    [`${Math.round(plan.fuel_plan.fluid_target_ml_per_hour || 0)} mL/h`, "Fluid Target"],
  ];

  if (plan.longest_climb) {
    cards.push([`${plan.longest_climb.length_km.toFixed(1)} km`, "Longest Climb"]);
  }
  if (plan.profile && plan.profile.w_per_kg > 0) {
    cards.push([`${plan.profile.w_per_kg.toFixed(2)} W/kg`, "FTP W/kg"]);
  }
  if (plan.profile && plan.profile.weekly_hours > 0) {
    cards.push([`${plan.profile.weekly_hours.toFixed(1)} h/wk`, "Recent Volume"]);
  }
  if (plan.profile && plan.profile.longest_recent_ride_km > 0) {
    cards.push([`${Math.round(plan.profile.longest_recent_ride_km)} km`, "Longest Recent Ride"]);
  }

  cards.forEach(([value, label]) => raceMetricGrid.appendChild(createMetricCard(value, label)));
}

function renderClimbs(climbs) {
  climbOutput.replaceChildren();
  climbOutput.className = "stack";

  if (!Array.isArray(climbs) || climbs.length === 0) {
    climbOutput.classList.add("empty-state");
    climbOutput.textContent = "No selective climbs were detected from the route profile.";
    return;
  }

  climbs.forEach((climb) => {
    climbOutput.appendChild(
      createInfoCard(
        climb.name || "Climb",
        `km ${climb.start_km.toFixed(1)}-${climb.end_km.toFixed(1)} • +${Math.round(climb.gain_m)} m • ${climb.avg_grade_pct.toFixed(1)}% avg • ${climb.max_grade_pct.toFixed(1)}% max`,
        `${cleanLabel(climb.severity)} climb. Estimated duration ${formatDuration(climb.estimated_duration_seconds || 0)}.`,
        `${climb.length_km.toFixed(1)} km`,
      ),
    );
  });
}

function renderPressurePoints(points) {
  pressureOutput.replaceChildren();
  pressureOutput.className = "stack";

  if (!Array.isArray(points) || points.length === 0) {
    pressureOutput.classList.add("empty-state");
    pressureOutput.textContent = "No standout route pressure points were detected.";
    return;
  }

  points.forEach((point) => {
    pressureOutput.appendChild(
      createInfoCard(
        `${point.title} • km ${point.distance_km.toFixed(1)}`,
        `${cleanLabel(point.category)} • ${cleanLabel(point.severity)}`,
        `${point.reason} ${point.action}`,
      ),
    );
  });
}

function renderFuelPlan(fuelPlan) {
  fuelOutput.replaceChildren();
  fuelOutput.className = "stack";

  if (!fuelPlan) {
    fuelOutput.classList.add("empty-state");
    fuelOutput.textContent = "Fueling details will appear here after the route is analyzed.";
    return;
  }

  const summary = document.createElement("article");
  summary.className = "structure-summary";
  const strong = document.createElement("strong");
  strong.textContent = `${Math.round(fuelPlan.carb_target_g_per_hour || 0)} g/h carbs • ${Math.round(fuelPlan.fluid_target_ml_per_hour || 0)} mL/h fluid`;
  summary.appendChild(strong);
  const meta = document.createElement("div");
  meta.className = "block-meta";
  meta.textContent = `Start fueling by ${formatDuration(fuelPlan.start_fuel_by_seconds || 0)} or km ${(fuelPlan.start_fuel_by_distance_km || 0).toFixed(1)} • Est. total carbs ${Math.round(fuelPlan.estimated_total_carb_g || 0)} g`;
  summary.appendChild(meta);
  fuelOutput.appendChild(summary);

  if (fuelPlan.caffeine_plan) {
    fuelOutput.appendChild(createInfoCard("Caffeine", "", fuelPlan.caffeine_plan));
  }

  (fuelPlan.checkpoints || []).forEach((checkpoint) => {
    fuelOutput.appendChild(
      createInfoCard(
        checkpoint.title,
        `km ${checkpoint.distance_km.toFixed(1)} • ${formatDuration(checkpoint.approx_time_seconds)}`,
        checkpoint.action,
      ),
    );
  });
}

function renderDecisiveSectors(sectors) {
  sectorOutput.replaceChildren();
  sectorOutput.className = "stack";

  if (!Array.isArray(sectors) || sectors.length === 0) {
    sectorOutput.classList.add("empty-state");
    sectorOutput.textContent = "No single decisive sector stands out from the route alone.";
    return;
  }

  sectors.forEach((sector) => {
    sectorOutput.appendChild(
      createInfoCard(
        `${sector.title} • km ${sector.start_km.toFixed(1)}-${sector.end_km.toFixed(1)}`,
        cleanLabel(sector.type),
        `${sector.why_it_matters} ${sector.recommended_action}`,
      ),
    );
  });
}

function renderRacePlan(plan) {
  raceResultsShell.classList.toggle("is-ready", !!plan);
  renderRaceMetricGrid(plan);

  if (!plan) {
    courseTitle.textContent = "No race plan yet";
    courseMeta.textContent = "Upload a course FIT to generate the route summary and tactical script.";
    resetStack(climbOutput, "Build a race plan to identify the selective climbs.");
    resetStack(pressureOutput, "Route pressure points and squeeze markers will appear here.");
    resetStack(fuelOutput, "Fuel checkpoints will appear here once the course is analyzed.");
    resetStack(sectorOutput, "The tool will rank likely selection and attack zones from the route profile.");
    return;
  }

  courseTitle.textContent = plan.course_name || "Race plan";
  const metaParts = [
    `${cleanLabel(plan.source_type)} FIT`,
    plan.profile && plan.profile.strategy_mode ? `${cleanLabel(plan.profile.strategy_mode)} mode` : "",
    plan.profile && plan.profile.goal ? cleanLabel(plan.profile.goal) : "",
    plan.sport ? cleanLabel(plan.sport) : "",
  ].filter(Boolean);
  courseMeta.textContent = metaParts.join(" • ");

  renderClimbs(plan.climbs);
  renderPressurePoints(plan.pressure_points);
  renderFuelPlan(plan.fuel_plan);
  renderDecisiveSectors(plan.decisive_sectors);
}

function resetRaceResults() {
  renderRacePlan(null);
  renderWarnings(raceWarningsList, []);
  renderArtifacts(raceArtifactList, [], "Run a race plan to build the route bundle.");
  setSummary(raceSummaryOutput, raceCopyBtn, "");
}

function validateRaceInputs() {
  const hasFile = !!selectedCourseFile;
  racePlanBtn.disabled = !hasFile;

  if (!hasFile) {
    setStatus(raceStatusEl, "Idle. Select a course .fit file.");
    return;
  }

  const ftp = positiveNumber(raceFTPInput);
  const weight = positiveNumber(raceWeightInput);
  const mode = cleanLabel(strategyInput.value || "balanced");
  const parts = [mode];
  const goal = cleanLabel(goalInput.value || "");
  const weeklyKM = positiveNumber(weeklyKMInput);
  if (ftp > 0) {
    parts.push(`FTP ${Math.round(ftp)} W`);
  }
  if (weight > 0) {
    parts.push(`weight ${weight.toFixed(1)} kg`);
  }
  if (goal !== "Unknown") {
    parts.push(goal);
  }
  if (weeklyKM > 0) {
    parts.push(`${Math.round(weeklyKM)} km/week`);
  }
  let message = `Ready to plan with ${parts.join(" • ")}.`;
  let tone = "";
  if (importedProfileWarning) {
    message += ` Import note: ${importedProfileWarning}`;
    tone = "warning";
  }
  setStatus(raceStatusEl, message, tone);
}

function setSelectedCourseFile(file) {
  selectedCourseFile = file || null;
  raceZipBlob = null;
  raceDownloadBtn.disabled = true;
  resetRaceResults();

  if (!selectedCourseFile) {
    courseDropLabel.textContent = "Drag and drop a course .fit file, or click to browse";
    courseFileMeta.textContent = "No course file selected.";
    validateRaceInputs();
    return;
  }

  courseDropLabel.textContent = selectedCourseFile.name;
  courseFileMeta.textContent = `Selected: ${selectedCourseFile.name} (${(selectedCourseFile.size / 1024).toFixed(1)} KB)`;
  validateRaceInputs();
}

async function applyProfileImport(file) {
  if (!file) {
    importedProfileWarning = "";
    profileImportMeta.textContent = "Optional. Import a Scout export to prefill recent training volume.";
    validateRaceInputs();
    return;
  }

  try {
    const raw = await file.text();
    const payload = JSON.parse(raw);
    const imported = extractImportedProfile(payload);
    if (!imported) {
      throw new Error("Unsupported profile payload");
    }

    setNumberInput(raceFTPInput, imported.ftp);
    setNumberInput(raceWeightInput, imported.weight, 1);
    setNumberInput(carbInput, imported.carbs);
    setNumberInput(bottleInput, imported.bottleML);
    if (imported.startBottles !== null && imported.startBottles >= 0) {
      bottlesInput.value = String(Math.round(imported.startBottles));
    }
    setNumberInput(caffeineInput, imported.caffeine, 1);
    setNumberInput(weeklyHoursInput, imported.weeklyHours, 1);
    setNumberInput(weeklyKMInput, imported.weeklyKM);
    setNumberInput(longestRideInput, imported.longestRideKM);
    setSelectInput(goalInput, imported.goal, ["finish", "lead_group", "top_10", "podium", "win", "support_teammate"]);
    setSelectInput(riderTypeInput, imported.riderType, ["climber", "puncheur", "all_rounder", "diesel_rouleur", "steady_endurance_rider", "sprinter"]);
    setSelectInput(teamSupportInput, imported.teamSupport, ["solo", "teammates"]);
    setSelectInput(technicalInput, imported.technical, ["low", "medium", "high"]);
    setSelectInput(strategyInput, imported.strategyMode, ["balanced", "conservative", "aggressive"]);

    const summaryBits = [];
    if (imported.athleteName) {
      summaryBits.push(imported.athleteName);
    }
    if (imported.weeklyHours > 0) {
      summaryBits.push(`${imported.weeklyHours.toFixed(1)} h/week`);
    }
    if (imported.weeklyKM > 0) {
      summaryBits.push(`${Math.round(imported.weeklyKM)} km/week`);
    }
    if (imported.longestRideKM > 0) {
      summaryBits.push(`long ride ${Math.round(imported.longestRideKM)} km`);
    }
    profileImportMeta.textContent = summaryBits.length > 0
      ? `Imported: ${summaryBits.join(" • ")}`
      : `Imported ${file.name}.`;
    importedProfileWarning = imported.warnings.length > 0 ? imported.warnings[0] : "";
    validateRaceInputs();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    importedProfileWarning = "";
    profileImportMeta.textContent = `Profile import failed: ${message}`;
    setStatus(raceStatusEl, `Profile import failed: ${message}`, "error");
  }
}

function postWorkerAction(action, buffer, options) {
  const id = ++requestCounter;
  worker.postMessage(
    {
      id,
      action,
      fileBuffer: buffer,
      options,
    },
    [buffer],
  );

  return new Promise((resolve) => {
    const onMessage = (event) => {
      if (!event.data || event.data.id !== id) {
        return;
      }
      worker.removeEventListener("message", onMessage);
      resolve(event.data);
    };
    worker.addEventListener("message", onMessage);
  });
}

async function runAnalysis() {
  if (!selectedFile) {
    return;
  }

  setStatus(statusEl, "Reading file...");
  resetRideResults();
  analyzeBtn.disabled = true;
  downloadBtn.disabled = true;
  copyBtn.disabled = true;

  try {
    const buffer = await selectedFile.arrayBuffer();
    setStatus(statusEl, "Analyzing in browser with the shared Go library...");

    const response = await postWorkerAction("analyze", buffer, {
      source_file_name: selectedFile.name,
      ftp_w: positiveNumber(ftpInput),
      weight_kg: positiveNumber(weightInput),
      format: "csv",
    });

    if (!response.ok) {
      setStatus(statusEl, `Analysis failed: ${response.error || "Unknown error"}`, "error");
      return;
    }

    zipBlob = new Blob([response.zipBuffer], { type: "application/zip" });
    const stem = selectedFile.name.replace(/\.fit$/i, "");
    zipName = `${stem || "fit"}_analysis.zip`;
    downloadBtn.disabled = false;

    const analysis = response.analysisJSON ? JSON.parse(response.analysisJSON) : null;
    renderAnalysis(analysis);
    renderWarnings(warningsList, Array.isArray(response.warnings) ? response.warnings : []);
    renderArtifacts(artifactList, Array.isArray(response.files) ? response.files : [], "Run an analysis to build the artifact bundle.");
    setSummary(summaryOutput, copyBtn, response.summary || "");

    const warningCount = Array.isArray(response.warnings) ? response.warnings.length : 0;
    setStatus(
      statusEl,
      warningCount > 0
        ? `Analysis complete with ${warningCount} warning${warningCount === 1 ? "" : "s"}.`
        : "Analysis complete. Summary and artifact bundle are ready.",
      "success",
    );
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(statusEl, `Analysis failed: ${message}`, "error");
  } finally {
    analyzeBtn.disabled = !selectedFile;
  }
}

async function runRacePlan() {
  if (!selectedCourseFile) {
    return;
  }

  setStatus(raceStatusEl, "Reading course file...");
  resetRaceResults();
  racePlanBtn.disabled = true;
  raceDownloadBtn.disabled = true;
  raceCopyBtn.disabled = true;

  try {
    const buffer = await selectedCourseFile.arrayBuffer();
    setStatus(raceStatusEl, "Building route-aware race plan in browser...");

    const response = await postWorkerAction("race-plan", buffer, {
      source_file_name: selectedCourseFile.name,
      ftp_w: positiveNumber(raceFTPInput),
      weight_kg: positiveNumber(raceWeightInput),
      max_carb_g_per_h: positiveNumber(carbInput),
      bottle_ml: positiveNumber(bottleInput),
      start_bottles: nonNegativeInteger(bottlesInput),
      caffeine_mg_per_kg: positiveNumber(caffeineInput),
      goal: goalInput.value || "",
      rider_type: riderTypeInput.value || "",
      weekly_hours: positiveNumber(weeklyHoursInput),
      weekly_km: positiveNumber(weeklyKMInput),
      longest_recent_ride_km: positiveNumber(longestRideInput),
      team_support: teamSupportInput.value || "",
      technical_confidence: technicalInput.value || "",
      strategy_mode: strategyInput.value || "balanced",
    });

    if (!response.ok) {
      setStatus(raceStatusEl, `Race plan failed: ${response.error || "Unknown error"}`, "error");
      return;
    }

    raceZipBlob = new Blob([response.zipBuffer], { type: "application/zip" });
    const stem = selectedCourseFile.name.replace(/\.fit$/i, "");
    raceZipName = `${stem || "course"}_race_plan.zip`;
    raceDownloadBtn.disabled = false;

    const plan = response.planJSON ? JSON.parse(response.planJSON) : null;
    renderRacePlan(plan);
    renderWarnings(raceWarningsList, Array.isArray(response.warnings) ? response.warnings : []);
    renderArtifacts(raceArtifactList, Array.isArray(response.files) ? response.files : [], "Run a race plan to build the route bundle.");
    setSummary(raceSummaryOutput, raceCopyBtn, response.summary || "");

    const warningCount = Array.isArray(response.warnings) ? response.warnings.length : 0;
    setStatus(
      raceStatusEl,
      warningCount > 0
        ? `Race plan complete with ${warningCount} note${warningCount === 1 ? "" : "s"}.`
        : "Race plan complete. Tactical script and export bundle are ready.",
      "success",
    );
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(raceStatusEl, `Race plan failed: ${message}`, "error");
  } finally {
    racePlanBtn.disabled = !selectedCourseFile;
  }
}

function downloadBlob(blob, name) {
  if (!blob) {
    return;
  }
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

async function copyTextFrom(textarea, statusTarget, message) {
  if (!textarea.value) {
    return;
  }
  try {
    await navigator.clipboard.writeText(textarea.value);
  } catch (_error) {
    textarea.focus();
    textarea.select();
    document.execCommand("copy");
  }
  setStatus(statusTarget, message, "success");
}

function wireDropZone(dropTarget, setter) {
  dropTarget.addEventListener("dragover", (event) => {
    event.preventDefault();
    dropTarget.classList.add("dragging");
  });
  dropTarget.addEventListener("dragleave", () => {
    dropTarget.classList.remove("dragging");
  });
  dropTarget.addEventListener("drop", (event) => {
    event.preventDefault();
    dropTarget.classList.remove("dragging");
    const file = event.dataTransfer.files && event.dataTransfer.files[0];
    setter(file);
  });
}

fileInput.addEventListener("change", (event) => {
  setSelectedRideFile(event.target.files && event.target.files[0]);
});

[ftpInput, weightInput].forEach((input) => {
  input.addEventListener("input", validateRideInputs);
});

analyzeBtn.addEventListener("click", runAnalysis);
downloadBtn.addEventListener("click", () => downloadBlob(zipBlob, zipName));
copyBtn.addEventListener("click", () => copyTextFrom(summaryOutput, statusEl, "Markdown summary copied."));

courseFileInput.addEventListener("change", (event) => {
  setSelectedCourseFile(event.target.files && event.target.files[0]);
});

[profileImportInput].forEach((input) => {
  input.addEventListener("change", (event) => {
    applyProfileImport(event.target.files && event.target.files[0]);
  });
});

[raceFTPInput, raceWeightInput, carbInput, bottleInput, bottlesInput, caffeineInput, goalInput, riderTypeInput, weeklyHoursInput, weeklyKMInput, longestRideInput, teamSupportInput, technicalInput, strategyInput].forEach((input) => {
  input.addEventListener("input", validateRaceInputs);
  input.addEventListener("change", validateRaceInputs);
});

racePlanBtn.addEventListener("click", runRacePlan);
raceDownloadBtn.addEventListener("click", () => downloadBlob(raceZipBlob, raceZipName));
raceCopyBtn.addEventListener("click", () => copyTextFrom(raceSummaryOutput, raceStatusEl, "Race plan copied."));

wireDropZone(dropZone, setSelectedRideFile);
wireDropZone(courseDropZone, setSelectedCourseFile);

resetRideResults();
validateRideInputs();
resetRaceResults();
validateRaceInputs();
