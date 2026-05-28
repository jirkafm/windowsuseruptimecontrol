const days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const fullDays = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];
let weeklyState = null;
let draft = [];

const $ = (id) => document.getElementById(id);

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
  $("week-range").textContent = `${state.week_start} to ${weekEnd(state.week_start)}`;
  $("weekly-used").textContent = formatDuration(weeklyConsumed(state));
  $("weekly-remaining").textContent = formatDuration(state.remaining_sec);
  $("today-used").textContent = formatDuration(state.consumed_sec[idx]);
  $("today-remaining").textContent = formatDuration(Math.max(0, state.allocations_sec[idx] - state.consumed_sec[idx]));

  const pill = $("status-pill");
  pill.className = "pill";
  if (state.exhausted) {
    pill.textContent = "Week exhausted";
    pill.classList.add("exhausted");
  } else if (state.day_exhausted) {
    pill.textContent = "Day exhausted";
    pill.classList.add("warning");
  } else {
    pill.textContent = "Available";
  }
}

function renderChart(state) {
  const chart = $("weekly-chart");
  chart.textContent = "";
  const max = Math.max(...state.allocations_sec, ...state.consumed_sec, 900);
  days.forEach((day, idx) => {
    const wrap = document.createElement("div");
    wrap.className = "bar-wrap";
    const bars = document.createElement("div");
    bars.className = "bars";

    const allocated = document.createElement("div");
    allocated.className = "bar allocated";
    allocated.style.height = `${Math.max(2, (state.allocations_sec[idx] / max) * 150)}px`;
    allocated.title = `${fullDays[idx]} allocated ${formatDuration(state.allocations_sec[idx])}`;

    const consumed = document.createElement("div");
    consumed.className = "bar consumed";
    consumed.style.height = `${Math.max(2, (state.consumed_sec[idx] / max) * 150)}px`;
    consumed.title = `${fullDays[idx]} used ${formatDuration(state.consumed_sec[idx])}`;

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
    message = `Distribution total must equal ${formatDuration(weeklyState.weekly_allowance_sec)}. Current total is ${formatDuration(total)}.`;
  }
  draft.forEach((value, idx) => {
    if (!message && value > cap) message = `${fullDays[idx]} cannot exceed ${formatDuration(cap)}.`;
    if (!message && value < weeklyState.consumed_sec[idx]) message = `${fullDays[idx]} is below already used time.`;
  });

  status.textContent = message || "Distribution is valid.";
  status.className = message ? "invalid" : "";
  save.disabled = Boolean(message);
}

function renderForm(state) {
  const form = $("allocation-form");
  form.textContent = "";
  draft = [...state.allocations_sec];
  const cap = Math.floor(state.weekly_allowance_sec / 2);
  fullDays.forEach((day, idx) => {
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
    slider.setAttribute("aria-label", `${day} allocation`);

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
  $("form-status").textContent = "Saving...";
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
  renderStats(state);
  renderChart(state);
  renderForm(state);
}

async function load() {
  const response = await fetch("/user/api/status");
  if (!response.ok) {
    $("status-pill").textContent = "Unavailable";
    $("status-pill").className = "pill exhausted";
    $("form-status").textContent = await response.text();
    $("save-button").disabled = true;
    return;
  }
  render(await response.json());
}

$("save-button").addEventListener("click", saveDistribution);
load();
