// KlyraDB frontend — vanilla JS. Wails exposes go.main.App.*
const api = () => window.go && window.go.main && window.go.main.App;

const DB_LABELS = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  redis: "Redis",
};

const DB_CONN_URI = {
  postgres: (i) => `postgresql://${escape(i.user)}@localhost:${i.port}/postgres`,
  mysql:    (i) => `mysql://${escape(i.user)}@localhost:${i.port}`,
  mariadb:  (i) => `mariadb://${escape(i.user)}@localhost:${i.port}`,
  redis:    (_i) => `redis://localhost:${_i.port}`,
};

const state = {
  instances: [],
  versions: [],
  view: "instances",
  strings: {},
  locale: "en",
  dir: "ltr",
  filter: "all",
  selectedType: "postgres",
};

function t(key, ...args) {
  let s = state.strings[key] ?? key;
  args.forEach((a) => (s = s.replace("%s", a)));
  return s;
}

// ---------- i18n apply ----------
function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.getAttribute("data-i18n"));
  });
  document.documentElement.lang = state.locale;
  document.documentElement.dir = state.dir || "ltr";
  const nameInput = document.getElementById("f-name");
  if (nameInput) nameInput.placeholder = t("modal.name_ph");
  updateTitles();
}

function updateTitles() {
  const titles = {
    instances: ["title.instances", "sub.instances"],
    versions:  ["title.versions",  "sub.versions"],
    logs:      ["title.logs",      "sub.logs"],
  };
  const [titleKey, subKey] = titles[state.view];
  document.getElementById("view-title").textContent = t(titleKey);
  document.getElementById("view-sub").textContent   = t(subKey);
}

// ---------- view switching ----------
document.querySelectorAll(".nav-item").forEach((btn) => {
  btn.addEventListener("click", () => switchView(btn.dataset.view));
});

function switchView(name) {
  state.view = name;
  document.querySelectorAll(".nav-item").forEach((b) =>
    b.classList.toggle("active", b.dataset.view === name)
  );
  document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
  document.getElementById(`view-${name}`).classList.remove("hidden");
  updateTitles();
  if (name === "versions") renderVersions();
}

// ---------- filter bar ----------
document.querySelectorAll(".filter-btn").forEach((btn) => {
  btn.addEventListener("click", () => {
    state.filter = btn.dataset.filter;
    document.querySelectorAll(".filter-btn").forEach((b) =>
      b.classList.toggle("active", b.dataset.filter === state.filter)
    );
    renderInstances();
  });
});

// ---------- data ----------
async function loadStrings() {
  state.strings = (await api().Strings()) || {};
  state.locale  = (await api().Locale())  || "en";
  try { state.dir = (await api().Direction()) || "ltr"; } catch { state.dir = "ltr"; }
  applyI18n();
}

async function refresh() {
  if (!api()) return;
  try {
    state.instances = (await api().ListInstances()) || [];
    state.versions  = (await api().ListVersions())  || [];
    renderInstances();
  } catch (e) {
    toast(e.message || String(e), true);
  }
}

// ---------- render instances ----------
function renderInstances() {
  const wrap  = document.getElementById("cards");
  const empty = document.getElementById("empty");
  wrap.innerHTML = "";

  const filtered = state.filter === "all"
    ? state.instances
    : state.instances.filter((i) => i.type === state.filter);

  if (!filtered.length) {
    empty.classList.remove("hidden");
    return;
  }
  empty.classList.add("hidden");

  filtered.forEach((i) => {
    const running = i.status === "running";
    const uri     = (DB_CONN_URI[i.type] || DB_CONN_URI.postgres)(i);
    const label   = DB_LABELS[i.type] || i.type;
    const el      = document.createElement("div");
    el.className  = "card" + (running ? " running" : "");
    el.innerHTML  = `
      <div class="card-head">
        <div class="card-name">
          <span class="status-dot ${i.status}"></span>
          ${escape(i.name)}
        </div>
        <div class="card-badges">
          <span class="db-badge ${i.type}">${label}</span>
          <span class="card-version">${t("card.version_prefix")}${escape(i.version)}</span>
        </div>
      </div>
      <div class="card-meta">
        <div class="meta-row">
          <span class="meta-label">${t("card.port")}</span>
          <span class="meta-value">${i.port}</span>
        </div>
        ${i.type !== "redis" ? `
        <div class="meta-row">
          <span class="meta-label">${t("card.user")}</span>
          <span class="meta-value">${escape(i.user)}</span>
        </div>` : ""}
        <div class="meta-row" style="grid-column: 1 / -1">
          <span class="meta-label">${t("card.conn")}</span>
          <span class="meta-value dim">${uri}</span>
        </div>
      </div>
      ${i.lastError ? `<div class="card-error">${escape(i.lastError)}</div>` : ""}
      <div class="card-actions">
        ${running
          ? `<button class="btn-mini" data-act="stop"  data-id="${i.id}">${t("btn.stop")}</button>`
          : `<button class="btn-mini" data-act="start" data-id="${i.id}">${t("btn.start")}</button>`}
        <button class="btn-mini" data-act="copy"   data-id="${i.id}">${t("btn.copy_uri")}</button>
        <button class="btn-mini danger" data-act="delete" data-id="${i.id}">${t("btn.delete")}</button>
      </div>
    `;
    wrap.appendChild(el);
  });

  wrap.querySelectorAll("button[data-act]").forEach((b) => {
    b.addEventListener("click", () => handleAction(b.dataset.act, b.dataset.id));
  });
}

async function handleAction(act, id) {
  const inst = state.instances.find((x) => x.id === id);
  try {
    if (act === "start") {
      await api().StartInstance(id);
      toast(t("toast.started"));
    } else if (act === "stop") {
      await api().StopInstance(id);
      toast(t("toast.stopped"));
    } else if (act === "delete") {
      if (!confirm(t("confirm.delete", inst.name))) return;
      await api().DeleteInstance(id);
      toast(t("toast.deleted"));
    } else if (act === "copy") {
      const uri = (DB_CONN_URI[inst.type] || DB_CONN_URI.postgres)(inst);
      await navigator.clipboard.writeText(uri);
      toast(t("toast.copied"));
      return;
    }
    await refresh();
  } catch (e) {
    toast(e.message || String(e), true);
  }
}

// ---------- versions view ----------
function renderVersions() {
  const wrap = document.getElementById("versions");
  wrap.innerHTML = "";

  const byType = {};
  state.versions.forEach((v) => {
    if (!byType[v.type]) byType[v.type] = [];
    byType[v.type].push(v);
  });

  const typeOrder = ["postgres", "mysql", "mariadb", "redis"];
  const installHintKey = {
    postgres: "ver.install_pg",
    mysql:    "ver.install_mysql",
    mariadb:  "ver.install_mariadb",
    redis:    "ver.install_redis",
  };

  typeOrder.forEach((type) => {
    const versions = byType[type];
    if (!versions || !versions.length) return;
    const label = DB_LABELS[type] || type;

    const group = document.createElement("div");
    group.className = "version-group";
    group.innerHTML = `<div class="version-group-header">
      <span class="db-dot ${type}"></span>
      <span class="version-group-label">${label}</span>
    </div>`;

    const grid = document.createElement("div");
    grid.className = "version-cards";

    versions.forEach((v) => {
      const card = document.createElement("div");
      card.className = "version-card";
      const hint = type === "postgres"
        ? `${t(installHintKey[type])}${escape(v.major)}`
        : t(installHintKey[type]);
      card.innerHTML = `
        <div class="major ${type}">${escape(v.major)}</div>
        <div>${v.installed
          ? `<span class="status-ok">● ${t("ver.installed")}</span>`
          : `<span class="status-missing">○ ${t("ver.not_installed")}</span>`}
        </div>
        ${v.installed
          ? `<code>${escape(v.binPath)}</code>`
          : `<code>${hint}</code>`}
      `;
      grid.appendChild(card);
    });

    group.appendChild(grid);
    wrap.appendChild(group);
  });
}

// ---------- modal ----------
const modal = document.getElementById("modal");
document.getElementById("btn-new").addEventListener("click", openModal);
document.getElementById("btn-empty-new").addEventListener("click", openModal);
document.getElementById("m-cancel").addEventListener("click", closeModal);
document.getElementById("m-create").addEventListener("click", submitCreate);

// DB type picker
document.querySelectorAll(".db-type-btn").forEach((btn) => {
  btn.addEventListener("click", async () => {
    document.querySelectorAll(".db-type-btn").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    state.selectedType = btn.dataset.type;
    await updateVersionSelect();
    await updatePortSuggestion();
  });
});

async function openModal() {
  state.selectedType = "postgres";
  document.querySelectorAll(".db-type-btn").forEach((b) =>
    b.classList.toggle("active", b.dataset.type === "postgres")
  );
  document.getElementById("f-name").value = "";
  document.getElementById("modal-error").classList.add("hidden");
  await updateVersionSelect();
  await updatePortSuggestion();
  modal.classList.remove("hidden");
  setTimeout(() => document.getElementById("f-name").focus(), 50);
}

async function updateVersionSelect() {
  const sel = document.getElementById("f-version");
  sel.innerHTML = "";
  const typeVersions = state.versions.filter((v) => v.type === state.selectedType);
  const installed = typeVersions.filter((v) => v.installed);
  const pool = installed.length ? installed : typeVersions;
  pool.forEach((v) => {
    const o    = document.createElement("option");
    o.value    = v.major;
    o.textContent = `${v.label}${v.installed ? "" : " (" + t("ver.not_installed") + ")"}`;
    o.disabled = !v.installed;
    sel.appendChild(o);
  });
}

async function updatePortSuggestion() {
  try {
    const port = await api().SuggestPort(state.selectedType);
    document.getElementById("f-port").value = port;
  } catch {
    const defaults = { postgres: 5432, mysql: 3306, mariadb: 3316, redis: 6379 };
    document.getElementById("f-port").value = defaults[state.selectedType] || 5432;
  }
}

function closeModal() { modal.classList.add("hidden"); }

async function submitCreate() {
  const name    = document.getElementById("f-name").value.trim();
  const version = document.getElementById("f-version").value;
  const port    = parseInt(document.getElementById("f-port").value, 10);
  const err     = document.getElementById("modal-error");

  if (!name) return showErr(err, t("error.name_required"));
  if (!version) return showErr(err, t("error.select_version"));

  const btn = document.getElementById("m-create");
  btn.disabled = true;
  try {
    await api().CreateInstance(name, state.selectedType, version, port);
    closeModal();
    await refresh();
    toast(t("toast.created"));
  } catch (e) {
    showErr(err, e.message || String(e));
  } finally {
    btn.disabled = false;
  }
}

function showErr(el, msg) {
  el.textContent = msg;
  el.classList.remove("hidden");
}

// ---------- helpers ----------
function escape(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

let toastTimer;
function toast(msg, isErr = false) {
  const el    = document.getElementById("toast");
  el.textContent = msg;
  el.className   = "toast" + (isErr ? " error" : "");
  el.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add("hidden"), 2500);
}

// ---------- boot ----------
window.addEventListener("DOMContentLoaded", () => {
  const tryBoot = async () => {
    if (api()) {
      await loadStrings();
      await refresh();
      setInterval(refresh, 4000);
    } else {
      setTimeout(tryBoot, 100);
    }
  };
  tryBoot();
});
