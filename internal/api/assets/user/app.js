const messages = {
  en: {
    title: "Weekly time",
    loadingWeek: "Loading current week...",
    loading: "Loading",
    language: "Language",
    statsLabel: "Weekly statistics",
    used: "Used",
    remaining: "Remaining",
    todayUsed: "Today used",
    todayLeft: "Today left",
    consumption: "Consumption",
    distribution: "Distribution",
    chartLabel: "Allocated and consumed time by day",
    saveDistribution: "Save distribution",
    weekExhausted: "Week exhausted",
    dayExhausted: "Day exhausted",
    available: "Available",
    unavailable: "Unavailable",
    saving: "Saving...",
    validDistribution: "Distribution is valid.",
    totalMismatch: (want, got) => `Distribution total must equal ${want}. Current total is ${got}.`,
    exceedsCap: (day, cap) => `${day} cannot exceed ${cap}.`,
    belowUsed: (day) => `${day} is below already used time.`,
    weekRange: (start, end) => `${start} to ${end}`,
    allocatedTitle: (day, value) => `${day} allocated ${value}`,
    consumedTitle: (day, value) => `${day} used ${value}`,
    allocationLabel: (day) => `${day} allocation`,
    daysShort: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
    daysFull: ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"],
  },
  cs: {
    title: "Týdenní čas",
    loadingWeek: "Načítá se aktuální týden...",
    loading: "Načítání",
    language: "Jazyk",
    statsLabel: "Týdenní statistiky",
    used: "Využito",
    remaining: "Zbývá",
    todayUsed: "Dnes využito",
    todayLeft: "Dnes zbývá",
    consumption: "Spotřeba",
    distribution: "Rozdělení",
    chartLabel: "Přidělený a využitý čas podle dne",
    saveDistribution: "Uložit rozdělení",
    weekExhausted: "Týden vyčerpán",
    dayExhausted: "Den vyčerpán",
    available: "Dostupné",
    unavailable: "Nedostupné",
    saving: "Ukládá se...",
    validDistribution: "Rozdělení je platné.",
    totalMismatch: (want, got) => `Součet rozdělení musí být ${want}. Aktuální součet je ${got}.`,
    exceedsCap: (day, cap) => `${day} nesmí překročit ${cap}.`,
    belowUsed: (day) => `${day} je pod již využitým časem.`,
    weekRange: (start, end) => `${start} až ${end}`,
    allocatedTitle: (day, value) => `${day}: přiděleno ${value}`,
    consumedTitle: (day, value) => `${day}: využito ${value}`,
    allocationLabel: (day) => `${day}: přidělení`,
    daysShort: ["Po", "Út", "St", "Čt", "Pá", "So", "Ne"],
    daysFull: ["Pondělí", "Úterý", "Středa", "Čtvrtek", "Pátek", "Sobota", "Neděle"],
  },
};

let lang = normalizeLanguage(localStorage.getItem("language") || navigator.language);
let weeklyState = null;
let draft = [];

const $ = (id) => document.getElementById(id);
const t = (key) => messages[lang][key];

function normalizeLanguage(value) {
  return String(value || "").toLowerCase().startsWith("cs") ? "cs" : "en";
}

function setText(id, value) {
  const element = $(id);
  if (element) element.textContent = value;
}

function applyLanguage() {
  document.documentElement.lang = lang;
  document.title = t("title");
  setText("page-title", t("title"));
  setText("language-label", t("language"));
  setText("stat-used-label", t("used"));
  setText("stat-remaining-label", t("remaining"));
  setText("stat-today-used-label", t("todayUsed"));
  setText("stat-today-left-label", t("todayLeft"));
  setText("consumption-title", t("consumption"));
  setText("distribution-title", t("distribution"));
  setText("save-button", t("saveDistribution"));
  $("stats")?.setAttribute("aria-label", t("statsLabel"));
  $("weekly-chart")?.setAttribute("aria-label", t("chartLabel"));
  $("language-select").value = lang;
}

function formatDuration(sec) {
  sec = Math.max(0, Number(sec) || 0);
  const hours = Math.floor(sec / 3600);
  const minutes = Math.floor((sec % 3600) / 60);
  if (hours === 0) return `${minutes}m`;
  if (minutes === 0) return `${hours}h`;
  return `${hours}h ${minutes}m`;
}

function weekEnd(weekStart) {
  const start = new Date(`${weekStart}T00:00:00`);
  const end = new Date(start);
  end.setDate(start.getDate() + 6);
  return end.toISOString().slice(0, 10);
}

function todayIndex() {
  const jsDay = new Date().getDay();
  return (jsDay + 6) % 7;
}

function weeklyConsumed(state) {
  return state.consumed_sec.reduce((sum, value) => sum + value, 0);
}

function renderStats(state) {
  const idx = todayIndex();
  $("week-range").textContent = t("weekRange")(state.week_start, weekEnd(state.week_start));
  $("weekly-used").textContent = formatDuration(weeklyConsumed(state));
  $("weekly-remaining").textContent = formatDuration(state.remaining_sec);
  $("today-used").textContent = formatDuration(state.consumed_sec[idx]);
  $("today-remaining").textContent = formatDuration(Math.max(0, state.allocations_sec[idx] - state.consumed_sec[idx]));

  const pill = $("status-pill");
  pill.className = "pill";
  if (state.exhausted) {
    pill.textContent = t("weekExhausted");
    pill.classList.add("exhausted");
  } else if (state.day_exhausted) {
    pill.textContent = t("dayExhausted");
    pill.classList.add("warning");
  } else {
    pill.textContent = t("available");
  }
}

function renderChart(state) {
  const chart = $("weekly-chart");
  chart.textContent = "";
  const max = Math.max(...state.allocations_sec, ...state.consumed_sec, 900);
  t("daysShort").forEach((day, idx) => {
    const wrap = document.createElement("div");
    wrap.className = "bar-wrap";
    const bars = document.createElement("div");
    bars.className = "bars";

    const allocated = document.createElement("div");
    allocated.className = "bar allocated";
    allocated.style.height = `${Math.max(2, (state.allocations_sec[idx] / max) * 150)}px`;
    allocated.title = t("allocatedTitle")(t("daysFull")[idx], formatDuration(state.allocations_sec[idx]));

    const consumed = document.createElement("div");
    consumed.className = "bar consumed";
    consumed.style.height = `${Math.max(2, (state.consumed_sec[idx] / max) * 150)}px`;
    consumed.title = t("consumedTitle")(t("daysFull")[idx], formatDuration(state.consumed_sec[idx]));

    const label = document.createElement("div");
    label.className = "day-label";
    label.textContent = day;

    bars.append(allocated, consumed);
    wrap.append(bars, label);
    chart.append(wrap);
  });
}

function validateDraft() {
  const total = draft.reduce((sum, value) => sum + value, 0);
  const cap = Math.floor(weeklyState.weekly_allowance_sec / 2);
  const status = $("form-status");
  const save = $("save-button");

  let message = "";
  if (total !== weeklyState.weekly_allowance_sec) {
    message = t("totalMismatch")(formatDuration(weeklyState.weekly_allowance_sec), formatDuration(total));
  }
  draft.forEach((value, idx) => {
    if (!message && value > cap) message = t("exceedsCap")(t("daysFull")[idx], formatDuration(cap));
    if (!message && value < weeklyState.consumed_sec[idx]) message = t("belowUsed")(t("daysFull")[idx]);
  });

  status.textContent = message || t("validDistribution");
  status.className = message ? "invalid" : "";
  save.disabled = Boolean(message);
}

function renderForm(state) {
  const form = $("allocation-form");
  form.textContent = "";
  draft = [...state.allocations_sec];
  const cap = Math.floor(state.weekly_allowance_sec / 2);
  t("daysFull").forEach((day, idx) => {
    const row = document.createElement("label");
    row.className = "day-row";

    const name = document.createElement("strong");
    name.textContent = day;

    const slider = document.createElement("input");
    slider.type = "range";
    slider.min = "0";
    slider.max = String(cap);
    slider.step = "900";
    slider.value = String(draft[idx]);
    slider.setAttribute("aria-label", t("allocationLabel")(day));

    const value = document.createElement("span");
    value.className = "day-meta";
    value.textContent = formatDuration(draft[idx]);

    slider.addEventListener("input", () => {
      draft[idx] = Number(slider.value);
      value.textContent = formatDuration(draft[idx]);
      validateDraft();
    });

    row.append(name, slider, value);
    form.append(row);
  });
  validateDraft();
}

async function saveDistribution() {
  $("save-button").disabled = true;
  $("form-status").textContent = t("saving");
  const response = await fetch("/user/api/distribution", {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({allocations_sec: draft}),
  });
  if (!response.ok) {
    $("form-status").textContent = await response.text();
    $("form-status").className = "invalid";
    validateDraft();
    return;
  }
  weeklyState = await response.json();
  render(weeklyState);
}

function render(state) {
  weeklyState = state;
  applyLanguage();
  renderStats(state);
  renderChart(state);
  renderForm(state);
}

async function load() {
  applyLanguage();
  const response = await fetch("/user/api/status");
  if (!response.ok) {
    $("status-pill").textContent = t("unavailable");
    $("status-pill").className = "pill exhausted";
    $("form-status").textContent = await response.text();
    $("save-button").disabled = true;
    return;
  }
  render(await response.json());
}

$("save-button").addEventListener("click", saveDistribution);
$("language-select").addEventListener("change", (event) => {
  lang = normalizeLanguage(event.target.value);
  localStorage.setItem("language", lang);
  if (weeklyState) {
    render(weeklyState);
  } else {
    applyLanguage();
  }
});
load();
