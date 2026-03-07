// 前端主入口：
// 1) 维护全局状态（服务列表、当前路由、文件管理、日志流）；
// 2) 负责路由切换与页面渲染；
// 3) 封装与后端 API 的交互。
(() => {
  "use strict";
  const S = {
    svcs: [],
    svc: null,
    view: "dashboard",
    tab: "overview",
    tree: true,
    collapsed: {},
    file: {
      path: "",
      parent: "",
      entries: [],
      sel: new Set(),
      focus: "",
      clip: [],
      paths: {},
    },
    editor: { path: "", saved: "", dirty: false },
    logs: { all: "", paused: false, timer: 0 },
    logHistory: { items: [], name: "", content: "" },
    deploy: { name: "", timer: 0 },
    qq: { name: "", timer: 0 },
    dashTimer: 0,
    metricsRefreshSeconds: 2,
    sign: [],
    lagrangeVersions: [],
    lagrangeDefaultVersion: "latest",
    confirm: null,
    confirmResolve: null,
    confirmMode: "confirm",
    entryHTTPWarnShown: false,
    miniTicker: 0,
    mini: { cpu: 0, mem: 0 },
    isSecure: false,
    disableHTTPSWarning: false,
    update: { checked: false, hasUpdate: false, result: null },
  };
  const $ = (id) => document.getElementById(id),
    $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const E = {};
  const IDS = [
    "authInit",
    "authLogin",
    "appShell",
    "initTitle",
    "initDesc",
    "initPassword",
    "initSubmit",
    "loginTitle",
    "loginDesc",
    "loginPassword",
    "loginSubmit",
    "brandTitle",
    "brandSub",
    "sidebarFloatToggle",
    "navDashboard",
    "navServices",
    "serviceTreeWrap",
    "serviceTree",
    "navDeploy",
    "navSettings",
    "globalHTTPWarn",
    "statusBar",
    "sidebarMiniTitle",
    "sidebarMiniTime",
    "sidebarMiniRunningLabel",
    "sidebarMiniRunningValue",
    "sidebarMiniUsageLabel",
    "sidebarMiniUsageValue",
    "sidebarMiniCpuLabel",
    "sidebarMiniMemLabel",
    "sidebarMiniCpuFill",
    "sidebarMiniMemFill",
    "refreshBtn",
    "logoutBtn",
    "shellTitle",
    "shellVersion",
    "shellSub",
    "globalCreateBtn",
    "dashboardView",
    "dashboardTitle",
    "dashboardSub",
    "dashboardToServices",
    "dashRunningLabel",
    "dashStoppedLabel",
    "dashErrorLabel",
    "dashTotalLabel",
    "dashSealLabel",
    "dashLagrangeLabel",
    "dashLLBotLabel",
    "dashRunning",
    "dashStopped",
    "dashError",
    "dashTotal",
    "dashSealCount",
    "dashLagrangeCount",
    "dashLLBotCount",
    "servicesView",
    "servicesListView",
    "servicesTitle",
    "servicesSub",
    "servicesCreateBtn",
    "serviceSearch",
    "serviceTypeFilter",
    "serviceStatusFilter",
    "serviceSort",
    "servicesEmpty",
    "serviceCards",
    "serviceDetailView",
    "detailBreadcrumb",
    "detailName",
    "detailMeta",
    "detailBackBtn",
    "detailStatusLabel",
    "detailPortLabel",
    "detailPidLabel",
    "detailStatusValue",
    "detailPortValue",
    "detailPidValue",
    "detailStartBtn",
    "detailStopBtn",
    "detailRestartBtn",
    "detailMoreBtn",
    "detailMoreMenu",
    "tabOverview",
    "tabLogs",
    "tabFiles",
    "tabConfig",
    "llbotNotice",
    "overviewPathLabel",
    "overviewPathValue",
    "overviewTypeLabel",
    "overviewTypeValue",
    "previewLogsTitle",
    "overviewLogPreview",
    "logsTitle",
    "logsRange",
    "logSearch",
    "logPauseBtn",
    "logHistoryBtn",
    "logDownloadBtn",
    "logClearBtn",
    "logsBox",
    "filesTitle",
    "filesSub",
    "fileSearch",
    "fileSort",
    "filesLockedNotice",
    "filesLockedText",
    "filesStopBtn",
    "fileUpBtn",
    "fileRefreshBtn",
    "fileUploadBtn",
    "fileNewDirBtn",
    "fileNewFileBtn",
    "fileCopyBtn",
    "filePasteBtn",
    "fileRenameBtn",
    "fileCompressBtn",
    "fileExtractBtn",
    "fileDeleteBtn",
    "fileUploadInput",
    "fileBreadcrumb",
    "fileSelectAll",
    "fileNameHead",
    "fileTypeHead",
    "fileSizeHead",
    "fileMtimeHead",
    "fileTableBody",
    "fileInfoTitle",
    "fileInfoBody",
    "configCardOneTitle",
    "configCardOneDesc",
    "openSettingsDrawerBtn",
    "configCardTwoTitle",
    "configCardTwoDesc",
    "openPortsDrawerBtn",
    "settingsView",
    "settingsTitle",
    "settingsSub",
    "settingsPlaceholder",
    "panelConfigTitle",
    "panelConfigDesc",
    "panelListenHostLabel",
    "panelListenHost",
    "panelListenPortLabel",
    "panelListenPort",
    "panelBasePathLabel",
    "panelBasePath",
    "panelSessionTTLLabel",
    "panelSessionTTL",
    "panelSessionMaxEntriesLabel",
    "panelSessionMaxEntries",
    "panelSessionCleanupIntervalLabel",
    "panelSessionCleanupInterval",
    "panelTrustProxyLabel",
    "panelTrustProxy",
    "panelTrustProxyText",
    "panelDisableHTTPSWarnLabel",
    "panelDisableHTTPSWarn",
    "panelDisableHTTPSWarnText",
    "panelLogRetentionLabel",
    "panelLogRetention",
    "panelLogRetentionDaysLabel",
    "panelLogRetentionDays",
    "panelLogMaxMBLabel",
    "panelLogMaxMB",
    "panelUpdateTitle",
    "panelUpdateCheckBtn",
    "panelUpdateApplyBtn",
    "panelUpdateStatus",
    "panelLogsClearBtn",
    "panelMetricsRefreshLabel",
    "panelMetricsRefresh",
    "panelConfigNotice",
    "panelConfigSaveBtn",
    "panelRawTitle",
    "panelRawMeta",
    "panelRawConfig",
    "panelRawSaveBtn",
    "toastStack",
    "createModal",
    "createTitle",
    "createSub",
    "createCloseBtn",
    "createBaseInfoTitle",
    "createTypeLabel",
    "createType",
    "createRegistryLabel",
    "createRegistry",
    "createDisplayLabel",
    "createDisplay",
    "createAutoStartLabel",
    "createAutoStart",
    "createTypeSpecific",
    "restartTitle",
    "restartEnabledLabel",
    "createRestartEnabled",
    "restartDelayLabel",
    "createRestartDelay",
    "restartMaxLabel",
    "createRestartMax",
    "createSummaryTitle",
    "createSummary",
    "createSubmitBtn",
    "settingsDrawer",
    "settingsDrawerTitle",
    "settingsDrawerCloseBtn",
    "detailDisplayLabel",
    "detailDisplayName",
    "detailAutoStart",
    "detailAutoStartLabel",
    "openPathWrap",
    "detailOpenPathLabel",
    "detailOpenPathUrl",
    "detailRestartEnabled",
    "detailRestartEnabledLabel",
    "detailRestartDelayLabel",
    "detailRestartDelay",
    "detailRestartMaxLabel",
    "detailRestartMax",
    "detailRestartCurrent",
    "detailLogPolicyTitle",
    "detailLogPolicyDesc",
    "detailLogRetentionLabel",
    "detailLogRetention",
    "detailLogRetentionDaysLabel",
    "detailLogRetentionDays",
    "detailLogMaxMBLabel",
    "detailLogMaxMB",
    "detailResetOpenPathBtn",
    "detailSettingsSaveBtn",
    "portsDrawer",
    "portsDrawerTitle",
    "portsDrawerCloseBtn",
    "portsContent",
    "portsSaveBtn",
    "editorModal",
    "editorTitle",
    "editorLang",
    "editorState",
    "editorFormatBtn",
    "editorSaveBtn",
    "editorCloseBtn",
    "editorText",
    "editorEncoding",
    "editorCursor",
    "deployLogModal",
    "deployLogTitle",
    "deployLogHint",
    "deployLogCloseBtn",
    "deployLogBox",
    "qqModal",
    "qqTitle",
    "qqCloseBtn",
    "qqStatusTitle",
    "qqStatusText",
    "qqInstallTitle",
    "qqUrlLabel",
    "qqUrl",
    "qqOverwriteText",
    "qqInstallBtn",
    "qqRefreshBtn",
    "qqLogsTitle",
    "qqLogBox",
    "qrModal",
    "qrTitle",
    "qrCloseBtn",
    "qrImage",
    "confirmModal",
    "confirmDialog",
    "confirmTitle",
    "confirmText",
    "confirmInputLabel",
    "confirmInput",
    "confirmCancelBtn",
    "confirmSubmitBtn",
    "logHistoryModal",
    "logHistoryTitle",
    "logHistoryCloseBtn",
    "logHistorySearch",
    "logHistoryClearBtn",
    "logHistoryClearAllBtn",
    "logHistoryTimeHead",
    "logHistorySizeHead",
    "logHistoryActionHead",
    "logHistoryBody",
    "logHistoryCurrentName",
    "logHistoryContent",
  ];
  const TYPES = ["Sealdice", "Lagrange", "LuckyLilliaBot"];
  const TXT = [
    ".txt",
    ".json",
    ".yaml",
    ".yml",
    ".toml",
    ".ini",
    ".cfg",
    ".conf",
    ".js",
    ".ts",
    ".sh",
    ".md",
    ".xml",
    ".html",
    ".css",
    ".env",
    ".log",
  ];
  const ARC = [".zip", ".tar", ".gz", ".tgz", ".rar", ".7z"];
  const LLN = () => tx("service.llbot.notice");
  const TXT_SAFE = (k, fb) => {
    const v = tx(k);
    return !v || v === k ? fb : v;
  };
  function c() {
    IDS.forEach((id) => (E[id] = $(id)));
    [
      "dashSystemCpu",
      "dashSystemLoad",
      "dashSystemMem",
      "dashAppRSS",
      "dashAppData",
      "dashSystemCPUModel",
      "dashCpuTitle",
      "dashCpuDonut",
      "dashCpuMain",
      "dashCpuSub",
      "dashMemTitle",
      "dashMemDonut",
      "dashMemMain",
      "dashMemSub",
      "dashMemMeta",
      "dashDiskTitle",
      "dashDiskDonut",
      "dashDiskMain",
      "dashDiskSub",
      "dashDiskMeta1",
      "dashDiskMeta2",
      "dashSummaryBar",
      "detailForceBtn",
      "detailEntryBtn",
      "detailConfigBtn",
      "detailStatusPill",
    "overviewRssValue",
    "overviewInstallSizeValue",
    "overviewLogSizeValue",
    "overviewRssLabel",
    "overviewInstallSizeLabel",
    "overviewLogSizeLabel",
      "settingsPortSection",
      "settingsActionSection",
      "navServicesToggle",
      "editorMount",
    ].forEach((id) => (E[id] = $(id)));
    [
      "navDeploy",
      "globalCreateBtn",
      "openSettingsDrawerBtn",
      "openPortsDrawerBtn",
      "editorFormatBtn",
    ].forEach((id) => {
      if (!E[id]) E[id] = document.createElement("button");
    });
  }
  function basePrefix() {
    const parts = window.location.pathname.split("/").filter(Boolean);
    const roots = ["dashboard", "settings", "services"];
    const i = parts.findIndex((part) => roots.includes(part));
    if (i >= 0) return "/" + parts.slice(0, i).join("/");
    if (parts.length === 1 && !parts[0].includes(".")) return "/" + parts[0];
    return "";
  }
  function appPath(p) {
    const clean = p.startsWith("/") ? p : "/" + p;
    const base = basePrefix();
    return base && base !== "/" ? base + clean : clean;
  }
  function ep(p) {
    const clean = p.startsWith("api/")
      ? "/" + p
      : "/api/" + p.replace(/^\/+/, "");
    return appPath(clean);
  }
  function trMessage(raw) {
    if (typeof raw !== "string") return raw;
    const key = raw.trim();
    if (!key) return raw;
    const translated =
      typeof window.text === "function" ? window.text(key, key) : key;
    return translated && translated !== key ? translated : raw;
  }
  async function j(p, o = {}) {
    const r = await fetch(ep(p), {
      credentials: "same-origin",
      ...o,
      headers: {
        ...(o.body instanceof FormData
          ? {}
          : { "Content-Type": "application/json" }),
        ...(o.headers || {}),
      },
    });
    const t = r.headers.get("content-type") || "";
    const d = t.includes("application/json") ? await r.json() : await r.text();
    if (d && typeof d === "object" && typeof d.message === "string") {
      d.message = trMessage(d.message);
    }
    if (!r.ok) {
      const e = new Error(
        typeof d === "string" ? d : (d && d.message) || "HTTP " + r.status,
      );
      e.status = r.status;
      e.data = d;
      throw e;
    }
    return d;
  }
  const show =
    window.SealUIHelpers && window.SealUIHelpers.show
      ? window.SealUIHelpers.show
      : (e) => {
          if (e) e.classList.remove("hidden");
        };
  const hide =
    window.SealUIHelpers && window.SealUIHelpers.hide
      ? window.SealUIHelpers.hide
      : (e) => {
          if (e) e.classList.add("hidden");
        };
  const esc = (s) =>
    String(s ?? "").replace(
      /[&<>"']/g,
      (m) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[m],
    );
  const st = (s) =>
    s === "running"
      ? tx("service.status.running")
      : s === "stopped"
        ? tx("service.status.stopped")
        : tx("service.status.unknown");
  const disp = (s) => String(s.display_name || s.name || s.id || "").trim();
  const typ = (t) => t || tx("service.status.unknown");
  const tstr = (v) => {
    if (!v) return "-";
    const d = new Date(v);
    if (Number.isNaN(d)) return v;
    const p = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
  };
  const tx = (k, f = "") =>
    typeof window.text === "function" ? window.text(k, f) : f;
  window.tx = tx;
  const portValidation =
    window.SealPortValidation && window.SealPortValidation.createPortValidation
      ? window.SealPortValidation.createPortValidation({
          request: j,
          tx,
          getById: $,
          doc: document,
          get createSubmitBtn() {
            return E.createSubmitBtn;
          },
          get detailSettingsSaveBtn() {
            return E.detailSettingsSaveBtn;
          },
          get portsSaveBtn() {
            return E.portsSaveBtn;
          },
        })
      : null;
  const p = portValidation
    ? portValidation.parsePort
    : (v) => {
        const n = Number(v);
        if (!Number.isInteger(n) || n < 1 || n > 65535)
          throw new Error(tx("error.invalid_port"));
        return n;
      };
  const apiPortCheck = portValidation
    ? portValidation.apiPortCheck
    : async (port, serviceID = "") =>
        j("ports/check", {
          method: "POST",
          body: JSON.stringify({
            port: Number(port || 0),
            service_id: serviceID || "",
          }),
        });
  const setPortHint = portValidation
    ? portValidation.setPortHint
    : (input, hint, msg, isErr) => {
        if (!input) return;
        input.classList.toggle("input-invalid", !!isErr);
        if (hint) {
          hint.textContent = msg || "";
          hint.classList.toggle("error", !!isErr);
        }
      };
  const syncPortActionButtons = portValidation
    ? portValidation.syncPortActionButtons
    : () => {};
  const validatePortField = portValidation
    ? portValidation.validatePortField
    : async () => ({ ok: true });
  const bindPortFieldRealtime = portValidation
    ? portValidation.bindPortFieldRealtime
    : () => {};
  let settingsCenterApi = null;
  function initSettingsCenter() {
    const factory =
      window.SealSettingsCenter && window.SealSettingsCenter.create;
    if (typeof factory !== "function") return;
    const sc = factory({
      S,
      E,
      $,
      j,
      tx,
      esc,
      show,
      hide,
      loadSvcs,
      qqOpen,
      signOpts,
      srvOpts,
      probeCurrentSign,
      bindPortFieldRealtime,
      validatePortField,
      setPortHint,
      askConfirm,
      showWarning,
      toast,
    });
    settingsCenterApi = sc || null;
  }
  const bstr = (v) => {
    const n = Number(v || 0);
    if (!Number.isFinite(n) || n <= 0) return "0 B";
    const u = ["B", "KB", "MB", "GB", "TB"];
    let x = n,
      i = 0;
    while (x >= 1024 && i < u.length - 1) {
      x /= 1024;
      i++;
    }
    return `${x >= 100 || i === 0 ? x.toFixed(0) : x.toFixed(1)} ${u[i]}`;
  };
  function pct(n) {
    const v = Number(n || 0);
    return `${Math.max(0, Math.min(v, 100)).toFixed(0)}%`;
  }
  function ring(el, outerPct, outerColor, innerPct, innerColor) {
    if (!el) return;
    const mk = (r, w, pctValue, color, track) => {
      const c = 2 * Math.PI * r;
      const dash = (Math.max(0, Math.min(pctValue, 100)) / 100) * c;
      return `<circle class="donut-track" cx="66" cy="66" r="${r}" stroke="${track}" stroke-width="${w}"></circle><circle class="donut-progress" cx="66" cy="66" r="${r}" stroke="${color}" stroke-width="${w}" stroke-dasharray="${dash} ${Math.max(c - dash, 0)}"></circle>`;
    };
    el.innerHTML = `<svg class="donut-svg" viewBox="0 0 132 132" aria-hidden="true">${mk(46, 10, outerPct, outerColor, "rgba(148,163,184,0.16)")}${mk(32, 8, innerPct, innerColor, "rgba(148,163,184,0.10)")}</svg>`;
  }
  const near = (e) => e.scrollHeight - e.scrollTop - e.clientHeight < 24;
  function setPre(e, txt) {
    const k = near(e);
    e.textContent = txt || "";
    if (k) e.scrollTop = e.scrollHeight;
  }
  function toast(msg, kind = "info") {
    const d = document.createElement("div");
    d.className = "toast " + kind;
    d.textContent = msg;
    E.toastStack.appendChild(d);
    setTimeout(() => d.remove(), 3000);
    E.statusBar.textContent = msg;
  }
  function resolveConfirmDialog(result) {
    if (typeof S.confirmResolve === "function") {
      const done = S.confirmResolve;
      S.confirmResolve = null;
      done(result);
    }
    S.confirm = null;
    S.confirmMode = "confirm";
  }
  function closeConfirmDialog(result = false) {
    hide(E.confirmModal);
    resolveConfirmDialog(result);
  }
  function openConfirmDialog(opts) {
    const {
      title,
      message,
      requireInput = false,
      expectedInput = "",
      inputLabel = "",
      submitText,
      danger = true,
      showCancel = true,
      warning = false,
    } = opts || {};
    E.confirmTitle.textContent = title || (warning ? tx("warning.title") : tx("confirm.title"));
    E.confirmText.textContent = message || "";
    E.confirmInputLabel.textContent = requireInput
      ? (inputLabel || tx("confirm.input_label"))
      : "";
    E.confirmInput.value = "";
    E.confirmInput.classList.toggle("hidden", !requireInput);
    E.confirmInputLabel.classList.toggle("hidden", !requireInput);
    E.confirmCancelBtn.textContent = tx("action.close");
    E.confirmCancelBtn.classList.toggle("hidden", !showCancel);
    E.confirmSubmitBtn.textContent = submitText || tx("confirm.submit");
    E.confirmSubmitBtn.classList.remove(
      "btn-danger",
      "btn-primary",
      "btn-warn-outline",
    );
    E.confirmSubmitBtn.classList.toggle("btn-danger-outline", !!danger);
    E.confirmSubmitBtn.classList.toggle("btn-soft", !!warning);
    if (E.confirmDialog) {
      E.confirmDialog.classList.remove("confirm-danger", "confirm-warn");
      E.confirmDialog.classList.add(warning ? "confirm-warn" : "confirm-danger");
    }
    show(E.confirmModal);
    return new Promise((resolve) => {
      S.confirmResolve = resolve;
      S.confirm = async () => {
        if (requireInput && E.confirmInput.value.trim() !== expectedInput) {
          throw new Error(tx("error.delete_registry_mismatch"));
        }
        closeConfirmDialog(true);
      };
    });
  }
  async function askConfirm(message, title) {
    return openConfirmDialog({
      title,
      message,
      requireInput: false,
      danger: true,
      showCancel: true,
      warning: false,
    });
  }
  async function showWarning(message, title) {
    return openConfirmDialog({
      title: title || tx("warning.title"),
      message,
      requireInput: false,
      submitText: tx("warning.ack"),
      danger: false,
      showCancel: false,
      warning: true,
    });
  }
  function fill(sel, arr) {
    sel.innerHTML = arr
      .map(([v, l]) => `<option value="${esc(v)}">${esc(l)}</option>`)
      .join("");
  }
  function miniNow() {
    if (!E.sidebarMiniTime) return;
    E.sidebarMiniTime.textContent = new Date().toLocaleTimeString("zh-CN", {
      hour12: false,
    });
  }
  function miniUpdate() {
    miniNow();
    if (!E.sidebarMiniRunningValue) return;
    const run = S.svcs.filter((x) => x.status === "running").length;
    const total = S.svcs.length;
    const cpu = Math.max(0, Math.min(Number(S.mini.cpu || 0), 100));
    const mem = Math.max(0, Math.min(Number(S.mini.mem || 0), 100));
    E.sidebarMiniRunningValue.textContent = `${run}/${total}`;
    if (E.sidebarMiniUsageValue)
      E.sidebarMiniUsageValue.textContent = `CPU ${pct(cpu)} / MEM ${pct(mem)}`;
    if (E.sidebarMiniCpuFill)
      E.sidebarMiniCpuFill.style.width = `${cpu.toFixed(1)}%`;
    if (E.sidebarMiniMemFill)
      E.sidebarMiniMemFill.style.width = `${mem.toFixed(1)}%`;
  }
  function miniStart() {
    if (S.miniTicker) {
      clearInterval(S.miniTicker);
      S.miniTicker = 0;
    }
    miniUpdate();
    S.miniTicker = setInterval(() => {
      if (!document.hidden) miniNow();
    }, 1000);
  }
  function labels() {
    document.title = tx("app.title");
    E.initTitle.textContent = tx("auth.init.title");
    E.initDesc.textContent = tx("auth.init.desc");
    E.initSubmit.textContent = tx("auth.init.submit");
    E.loginTitle.textContent = tx("auth.login.title");
    E.loginDesc.textContent = tx("auth.login.desc");
    E.loginSubmit.textContent = tx("auth.login.submit");
    E.initPassword.placeholder = tx("auth.init.placeholder");
    E.loginPassword.placeholder = tx("auth.login.placeholder");
    if (E.sidebarFloatToggle)
      E.sidebarFloatToggle.title = tx("nav.sidebar_toggle");
    E.navDashboard.textContent = tx("nav.dashboard");
    E.navServices.textContent = tx("nav.services");
    E.navSettings.textContent = tx("nav.settings");
    if (E.sidebarMiniTitle)
      E.sidebarMiniTitle.textContent = tx("sidebar.quick.title");
    if (E.sidebarMiniRunningLabel)
      E.sidebarMiniRunningLabel.textContent = tx("sidebar.quick.running");
    if (E.sidebarMiniUsageLabel)
      E.sidebarMiniUsageLabel.textContent = tx("sidebar.quick.usage");
    if (E.sidebarMiniCpuLabel) E.sidebarMiniCpuLabel.textContent = "CPU";
    if (E.sidebarMiniMemLabel) E.sidebarMiniMemLabel.textContent = "MEM";
    E.refreshBtn.textContent = tx("action.refresh");
    E.logoutBtn.textContent = tx("action.logout");
    E.shellTitle.textContent = tx("app.title");
    if (E.shellVersion) E.shellVersion.textContent = "-";
    E.shellSub.textContent = tx("app.subtitle");
    E.dashboardTitle.textContent = tx("dashboard.title");
    E.dashboardSub.textContent = tx("dashboard.subtitle");
    E.dashboardToServices.textContent = tx("dashboard.goto_services");
    E.dashRunningLabel.textContent = tx("dashboard.running");
    E.dashStoppedLabel.textContent = tx("dashboard.stopped");
    if (E.dashErrorLabel) E.dashErrorLabel.textContent = tx("dashboard.error");
    E.dashTotalLabel.textContent = tx("dashboard.total");
    E.dashSealLabel.textContent = tx("dashboard.sealdice");
    E.dashLagrangeLabel.textContent = tx("dashboard.lagrange");
    E.dashLLBotLabel.textContent = tx("dashboard.llbot");
    if (E.dashCpuTitle) E.dashCpuTitle.textContent = tx("dashboard.card.cpu");
    if (E.dashMemTitle)
      E.dashMemTitle.textContent = tx("dashboard.card.memory");
    if (E.dashDiskTitle)
      E.dashDiskTitle.textContent = tx("dashboard.card.disk");
    E.servicesTitle.textContent = tx("service.overview.title");
    E.servicesSub.textContent = tx("service.overview.subtitle");
    E.servicesCreateBtn.textContent = tx("service.create");
    E.serviceSearch.placeholder = tx("service.search.placeholder");
    E.detailBackBtn.textContent = tx("detail.back");
    E.detailStatusLabel.textContent = tx("detail.hero.status");
    E.detailPortLabel.textContent = tx("detail.hero.port");
    E.detailPidLabel.textContent = tx("detail.hero.pid");
    E.detailStartBtn.textContent = tx("detail.start");
    E.detailStopBtn.textContent = tx("detail.stop");
    E.detailRestartBtn.textContent = tx("event.restart");
    E.detailMoreBtn.textContent = tx("action.more");
    E.overviewPathLabel.textContent = tx("detail.overview.path");
    E.overviewTypeLabel.textContent = tx("service.type");
    E.previewLogsTitle.textContent = tx("detail.overview.recent_logs");
    E.logsTitle.textContent = tx("detail.logs.title");
    E.logsRange.textContent = tx("detail.logs.range")
      .replace("{from}", "-")
      .replace("{to}", "-");
    E.logPauseBtn.textContent = tx("detail.logs.pause");
    E.logHistoryBtn.textContent = tx("detail.logs.history");
    E.logDownloadBtn.textContent = tx("file.download");
    E.logClearBtn.textContent = tx("action.refresh");
    E.logSearch.placeholder = tx("detail.logs.filter_placeholder");
    E.filesTitle.textContent = tx("file.title");
    E.filesSub.textContent = tx("detail.files.inline");
    E.filesLockedText.textContent = tx("file.warning.live_edit");
    E.filesStopBtn.textContent = tx("detail.stop");
    E.fileUpBtn.textContent = tx("file.nav.up");
    E.fileRefreshBtn.textContent = tx("file.nav.refresh");
    E.fileUploadBtn.textContent = tx("file.upload");
    E.fileNewDirBtn.textContent = tx("file.new_dir");
    E.fileNewFileBtn.textContent = tx("file.new_file");
    E.fileCopyBtn.textContent = tx("file.copy");
    E.filePasteBtn.textContent = tx("file.paste");
    E.fileRenameBtn.textContent = tx("file.rename");
    E.fileCompressBtn.textContent = tx("file.compress");
    E.fileExtractBtn.textContent = tx("file.extract");
    E.fileDeleteBtn.textContent = tx("file.delete");
    E.fileSearch.placeholder = tx("file.search.placeholder");
    E.fileNameHead.textContent = tx("file.table.name");
    E.fileTypeHead.textContent = tx("file.table.type");
    E.fileSizeHead.textContent = tx("file.table.size");
    E.fileMtimeHead.textContent = tx("file.table.mtime");
    E.fileInfoTitle.textContent = tx("file.info.title");
    E.fileInfoBody.textContent = tx("file.info.empty");
    E.settingsTitle.textContent = tx("nav.settings");
    E.settingsSub.textContent = tx("settings.subtitle");
    E.settingsPlaceholder.textContent = tx("settings.placeholder");
    E.panelConfigTitle.textContent = tx("panel.settings.title");
    E.panelConfigDesc.textContent = tx("panel.settings.desc");
    E.panelListenHostLabel.textContent = tx("panel.settings.listen_host");
    E.panelListenPortLabel.textContent = tx("panel.settings.listen_port");
    E.panelBasePathLabel.textContent = tx("panel.settings.base_path");
    E.panelSessionTTLLabel.textContent = tx("panel.settings.session_ttl");
    if (E.panelTrustProxyLabel)
      E.panelTrustProxyLabel.textContent = tx("panel.settings.trust_proxy");
    if (E.panelTrustProxyText)
      E.panelTrustProxyText.textContent = tx(
        "panel.settings.trust_proxy_switch",
      );
    if (E.panelLogRetentionLabel)
      E.panelLogRetentionLabel.textContent = tx("panel.settings.log_retention");
    if (E.panelLogRetentionDaysLabel)
      E.panelLogRetentionDaysLabel.textContent = TXT_SAFE(
        "panel.settings.log_retention_days",
        "日志保留天数",
      );
    if (E.panelLogMaxMBLabel)
      E.panelLogMaxMBLabel.textContent = TXT_SAFE(
        "panel.settings.log_max_mb",
        "日志最大占用（MB）",
      );
    if (E.panelMetricsRefreshLabel)
      E.panelMetricsRefreshLabel.textContent = tx(
        "panel.settings.metrics_refresh",
      );
    const panelFileManagerEnabledLabel = $("panelFileManagerEnabledLabel");
    const panelFileManagerEnabledText = $("panelFileManagerEnabledText");
    const panelFileUploadMaxMBLabel = $("panelFileUploadMaxMBLabel");
    const panelLoginProtectEnabledLabel = $("panelLoginProtectEnabledLabel");
    const panelLoginProtectEnabledText = $("panelLoginProtectEnabledText");
    const panelLoginProtectMaxAttemptsLabel = $(
      "panelLoginProtectMaxAttemptsLabel",
    );
    const panelLoginProtectWindowLabel = $("panelLoginProtectWindowLabel");
    const panelLoginProtectBlockLabel = $("panelLoginProtectBlockLabel");
    const panelLoginProtectMaxBucketsLabel = $("panelLoginProtectMaxBucketsLabel");
    const panelLoginProtectBucketIdleTTLLabel = $("panelLoginProtectBucketIdleTTLLabel");
    const panelLoginProtectCleanupIntervalLabel = $("panelLoginProtectCleanupIntervalLabel");
    const panelSessionMaxEntriesLabel = $("panelSessionMaxEntriesLabel");
    const panelSessionCleanupIntervalLabel = $("panelSessionCleanupIntervalLabel");
    if (panelFileManagerEnabledLabel)
      panelFileManagerEnabledLabel.textContent = tx(
        "panel.settings.file_manager_enabled",
      );
    if (panelFileManagerEnabledText)
      panelFileManagerEnabledText.textContent = tx(
        "panel.settings.file_manager_enabled_switch",
      );
    if (panelFileUploadMaxMBLabel)
      panelFileUploadMaxMBLabel.textContent = tx(
        "panel.settings.file_upload_max_mb",
      );
    if (panelLoginProtectEnabledLabel)
      panelLoginProtectEnabledLabel.textContent = tx(
        "panel.settings.login_protect_enabled",
      );
    if (panelLoginProtectEnabledText)
      panelLoginProtectEnabledText.textContent = tx(
        "panel.settings.login_protect_enabled_switch",
      );
    if (E.panelDisableHTTPSWarnLabel)
      E.panelDisableHTTPSWarnLabel.textContent = tx(
        "panel.settings.disable_https_warning",
      );
    if (E.panelDisableHTTPSWarnText)
      E.panelDisableHTTPSWarnText.textContent = tx(
        "panel.settings.disable_https_warning_switch",
      );
    if (E.panelLogsClearBtn)
      E.panelLogsClearBtn.textContent = tx("panel.settings.logs_clear_all");
    if (E.panelUpdateTitle)
      E.panelUpdateTitle.textContent = TXT_SAFE("panel.update.title", "Update");
    if (E.panelUpdateCheckBtn)
      E.panelUpdateCheckBtn.textContent = TXT_SAFE(
        "panel.update.check",
        "Check Update",
      );
    if (E.panelUpdateApplyBtn)
      E.panelUpdateApplyBtn.textContent = TXT_SAFE(
        "panel.update.apply_and_restart",
        "Update And Restart",
      );
    if (E.panelUpdateStatus)
      E.panelUpdateStatus.textContent = TXT_SAFE(
        "panel.update.idle",
        "Press check update to fetch latest version info.",
      );
    if (panelLoginProtectMaxAttemptsLabel)
      panelLoginProtectMaxAttemptsLabel.textContent = tx(
        "panel.settings.login_protect_max_attempts",
      );
    if (panelLoginProtectWindowLabel)
      panelLoginProtectWindowLabel.textContent = tx(
        "panel.settings.login_protect_window_seconds",
      );
    if (panelLoginProtectBlockLabel)
      panelLoginProtectBlockLabel.textContent = tx(
        "panel.settings.login_protect_block_seconds",
      );
    if (panelLoginProtectMaxBucketsLabel)
      panelLoginProtectMaxBucketsLabel.textContent = tx(
        "panel.settings.login_protect_max_buckets",
      );
    if (panelLoginProtectBucketIdleTTLLabel)
      panelLoginProtectBucketIdleTTLLabel.textContent = tx(
        "panel.settings.login_protect_bucket_idle_ttl",
      );
    if (panelLoginProtectCleanupIntervalLabel)
      panelLoginProtectCleanupIntervalLabel.textContent = tx(
        "panel.settings.login_protect_cleanup_interval",
      );
    if (panelSessionMaxEntriesLabel)
      panelSessionMaxEntriesLabel.textContent = tx(
        "panel.settings.session_max_entries",
      );
    if (panelSessionCleanupIntervalLabel)
      panelSessionCleanupIntervalLabel.textContent = tx(
        "panel.settings.session_cleanup_interval",
      );
    updateHTTPSNotice();
    if (E.overviewRssLabel) E.overviewRssLabel.textContent = tx("detail.overview.rss");
    if (E.overviewInstallSizeLabel)
      E.overviewInstallSizeLabel.textContent = tx("detail.overview.install_size");
    if (E.overviewLogSizeLabel) E.overviewLogSizeLabel.textContent = tx("detail.overview.log_size");
    E.panelConfigSaveBtn.textContent = tx("panel.settings.save_form");
    E.panelRawTitle.textContent = tx("panel.settings.raw_title");
    E.panelRawSaveBtn.textContent = tx("panel.settings.save_raw");
    E.createTitle.textContent = tx("create.title");
    E.createSub.textContent = tx("create.subtitle");
    E.createCloseBtn.textContent = tx("action.close");
    if (E.createBaseInfoTitle)
      E.createBaseInfoTitle.textContent = TXT_SAFE(
        "create.group.basic",
        "服务基础信息",
      );
    E.createTypeLabel.textContent = tx("create.type.label");
    E.createRegistryLabel.textContent = tx("create.registry.label");
    E.createDisplayLabel.textContent = tx("create.display.label");
    E.createRegistry.placeholder = tx("create.registry.placeholder");
    E.createDisplay.placeholder = tx("create.display.placeholder");
    E.createAutoStartLabel.textContent = tx("create.autostart.label");
    E.restartTitle.textContent = tx("restart.title");
    E.restartEnabledLabel.textContent = tx("restart.enabled");
    E.restartDelayLabel.textContent = tx("restart.delay");
    E.restartMaxLabel.textContent = tx("restart.max_crash");
    E.createSummaryTitle.textContent = tx("create.step.4.title");
    E.createSubmitBtn.textContent = tx("create.submit");
    E.settingsDrawerTitle.textContent = tx("detail.config_center");
    E.settingsDrawerCloseBtn.textContent = tx("action.close");
    E.configCardOneTitle.textContent = tx(
      "detail.settings.section.general.title",
    );
    E.configCardOneDesc.textContent = tx(
      "detail.settings.section.general.desc",
    );
    E.configCardTwoTitle.textContent = tx(
      "detail.settings.section.restart.title",
    );
    E.configCardTwoDesc.textContent = tx(
      "detail.settings.section.restart.desc",
    );
    E.detailDisplayLabel.textContent = tx("detail.display_name");
    E.detailDisplayName.placeholder = tx("create.display.placeholder");
    E.detailAutoStartLabel.textContent = tx("detail.auto_start");
    E.detailOpenPathLabel.textContent = tx("detail.open_path.custom");
    E.detailOpenPathUrl.placeholder = tx("detail.open_path.custom.placeholder");
    E.detailRestartEnabledLabel.textContent = tx("restart.enabled");
    E.detailRestartDelayLabel.textContent = tx("restart.delay");
    E.detailRestartMaxLabel.textContent = tx("restart.max_crash");
    if (E.detailLogPolicyTitle)
      E.detailLogPolicyTitle.textContent = tx("detail.logs.policy.title");
    if (E.detailLogPolicyDesc)
      E.detailLogPolicyDesc.textContent = tx("detail.logs.policy.desc");
    if (E.detailLogRetentionLabel)
      E.detailLogRetentionLabel.textContent = tx("panel.settings.log_retention");
    if (E.detailLogRetentionDaysLabel)
      E.detailLogRetentionDaysLabel.textContent = tx(
        "panel.settings.log_retention_days",
      );
    if (E.detailLogMaxMBLabel)
      E.detailLogMaxMBLabel.textContent = tx("panel.settings.log_max_mb");
    E.detailResetOpenPathBtn.textContent = tx("detail.open_path.reset");
    E.detailSettingsSaveBtn.textContent = tx("detail.settings.save");
    E.editorTitle.textContent = tx("editor.title");
    E.editorSaveBtn.textContent = tx("editor.save");
    E.editorCloseBtn.textContent = tx("action.close");
    E.editorEncoding.textContent = tx("editor.encoding");
    E.deployLogTitle.textContent = tx("deploy.logs.title");
    E.deployLogHint.textContent = tx("deploy.logs.waiting");
    E.deployLogCloseBtn.textContent = tx("action.close");
    E.qqTitle.textContent = tx("qq.modal.title");
    E.qqCloseBtn.textContent = tx("action.close");
    E.qqStatusTitle.textContent = tx("qq.modal.step.status");
    E.qqInstallTitle.textContent = tx("qq.modal.step.install");
    E.qqUrlLabel.textContent = tx("qq.modal.url.label");
    E.qqOverwriteText.textContent = tx("qq.modal.overwrite");
    E.qqInstallBtn.textContent = tx("qq.modal.install");
    E.qqRefreshBtn.textContent = tx("action.refresh");
    E.qqLogsTitle.textContent = tx("qq.modal.step.logs");
    E.qrTitle.textContent = tx("detail.qr.title");
    E.qrCloseBtn.textContent = tx("action.close");
    E.confirmTitle.textContent = tx("confirm.title");
    E.confirmInputLabel.textContent = tx("confirm.input_label");
    E.confirmCancelBtn.textContent = tx("action.close");
    E.confirmSubmitBtn.textContent = tx("confirm.submit");
    if (E.logHistoryTitle) E.logHistoryTitle.textContent = tx("detail.logs.history");
    if (E.logHistoryCloseBtn) E.logHistoryCloseBtn.textContent = tx("action.close");
    if (E.logHistoryClearBtn) E.logHistoryClearBtn.textContent = tx("detail.logs.clear_selected");
    if (E.logHistoryClearAllBtn) E.logHistoryClearAllBtn.textContent = tx("detail.logs.clear_all");
    if (E.logHistorySearch)
      E.logHistorySearch.placeholder = tx("detail.logs.filter_placeholder");
    if (E.logHistoryTimeHead) E.logHistoryTimeHead.textContent = tx("detail.logs.created_at");
    if (E.logHistorySizeHead) E.logHistorySizeHead.textContent = tx("file.table.size");
    if (E.logHistoryActionHead) E.logHistoryActionHead.textContent = tx("detail.logs.actions");
    fill(E.serviceTypeFilter, [
      ["all", tx("service.filter.all_types")],
      ["Sealdice", "Sealdice"],
      ["Lagrange", "Lagrange"],
      ["LuckyLilliaBot", "LuckyLilliaBot"],
    ]);
    fill(E.serviceStatusFilter, [
      ["all", tx("service.filter.all_status")],
      ["running", tx("service.status.running")],
      ["stopped", tx("service.status.stopped")],
    ]);
    fill(E.serviceSort, [
      ["display", tx("service.sort.name")],
      ["type", tx("service.type")],
      ["status", tx("service.sort.status")],
      ["updated", tx("service.sort.updated")],
    ]);
    fill(E.fileSort, [
      ["name", tx("service.sort.name")],
      ["size", tx("file.table.size")],
      ["time", tx("file.table.mtime")],
      ["type", tx("file.table.type")],
    ]);
    fill(E.createType, [
      ["Sealdice", tx("create.type.sealdice")],
      ["Lagrange", tx("create.type.lagrange")],
      ["LuckyLilliaBot", tx("create.type.llbot")],
    ]);
    $$(".tab-btn").forEach(
      (b) => (b.textContent = tx("detail.tab." + b.dataset.tab)),
    );
    updateHTTPSNotice();
  }
  function updateHTTPSNotice() {
    const showWarn = !S.isSecure && !S.disableHTTPSWarning;
    if (E.globalHTTPWarn) {
      E.globalHTTPWarn.textContent = tx("status.https_warn");
      E.globalHTTPWarn.classList.toggle("hidden", !showWarn);
    }
    if (E.statusBar) {
      E.statusBar.textContent = "";
    }
    if (E.panelConfigNotice) {
      E.panelConfigNotice.textContent = tx("panel.settings.restart_notice");
    }
  }
  function quickPanelVisible() {
    if (!E.appShell || E.appShell.classList.contains("hidden")) return false;
    if (window.innerWidth <= 1100)
      return E.appShell.classList.contains("sidebar-mobile-open");
    return !E.appShell.classList.contains("sidebar-collapsed");
  }
  function shouldRefreshMetrics() {
    if (document.hidden) return false;
    if (S.view === "dashboard") return true;
    return quickPanelVisible();
  }
  function dashPolling() {
    if (S.dashTimer) {
      clearInterval(S.dashTimer);
      S.dashTimer = 0;
    }
    const ms = Math.max(1000, Number(S.metricsRefreshSeconds || 2) * 1000);
    S.dashTimer = setInterval(() => {
      if (shouldRefreshMetrics()) dashMetrics();
    }, ms);
  }
  function view(v) {
    S.view = v;
    ["dashboardView", "servicesView", "settingsView"].forEach((id) =>
      hide(E[id]),
    );
    if (v === "dashboard") show(E.dashboardView);
    if (v === "services") show(E.servicesView);
    if (v === "settings") show(E.settingsView);
    [E.navDashboard, E.navServices, E.navSettings, E.navServicesToggle].forEach(
      (b) => {
        if (b) b.classList.remove("active");
      },
    );
    if (v === "dashboard") E.navDashboard.classList.add("active");
    if (v === "services") {
      E.navServices.classList.add("active");
      if (E.navServicesToggle) E.navServicesToggle.classList.add("active");
    }
    if (v === "settings") E.navSettings.classList.add("active");
    if (v === "settings") panelLoad();
    if (v === "dashboard") dashMetrics();
    dashPolling();
  }
  function dash() {
    const run = S.svcs.filter((s) => s.status === "running").length,
      stop = S.svcs.filter((s) => s.status === "stopped").length,
      err = S.svcs.filter((s) => s.status !== "stopped" && s.last_error).length;
    E.dashRunning.textContent = run;
    E.dashStopped.textContent = stop;
    if (E.dashError) E.dashError.textContent = err;
    E.dashTotal.textContent = S.svcs.length;
    E.dashSealCount.textContent = S.svcs.filter(
      (s) => s.type === "Sealdice",
    ).length;
    E.dashLagrangeCount.textContent = S.svcs.filter(
      (s) => s.type === "Lagrange",
    ).length;
    E.dashLLBotCount.textContent = S.svcs.filter(
      (s) => s.type === "LuckyLilliaBot",
    ).length;
  }
  async function dashMetrics() {
    try {
      const d = await j("metrics/overview");
      const sys = d.system || {},
        app = d.application || {};
      const nextRefresh = Math.max(1, Number(sys.metrics_refresh_seconds || 2));
      if (nextRefresh !== S.metricsRefreshSeconds) {
        S.metricsRefreshSeconds = nextRefresh;
        dashPolling();
      }
      const totalMem = Number(sys.mem_total_bytes || 0),
        availMem = Number(sys.mem_available_bytes || 0),
        usedMem = Math.max(totalMem - availMem, 0),
        swapTotal = Number(sys.swap_total_bytes || 0),
        swapFree = Number(sys.swap_free_bytes || 0),
        swapUsed = Math.max(swapTotal - swapFree, 0),
        diskTotal = Number(sys.disk_total_bytes || 0),
        diskFree = Number(sys.disk_free_bytes || 0),
        diskUsed = Number(
          sys.disk_used_bytes || Math.max(diskTotal - diskFree, 0),
        ),
        panelAndSvc = Number(app.total_rss_bytes || 0),
        otherMem = Number(app.other_used_bytes || 0),
        cpuPct = Math.max(0, Math.min(Number(sys.cpu_host_percent || 0), 100)),
        panelCpuPct = Math.max(
          0,
          Math.min(Number(sys.cpu_panel_percent || 0), 100),
        ),
        memPct =
          totalMem > 0
            ? Math.max(0, Math.min((usedMem / totalMem) * 100, 100))
            : 0,
        diskPct =
          diskTotal > 0
            ? Math.max(0, Math.min((diskUsed / diskTotal) * 100, 100))
            : 0,
        panelMemPct =
          totalMem > 0
            ? Math.max(0, Math.min((panelAndSvc / totalMem) * 100, 100))
            : 0,
        panelDiskPct =
          diskTotal > 0
            ? Math.max(
                0,
                Math.min(
                  (Number(app.data_size_bytes || 0) / diskTotal) * 100,
                  100,
                ),
              )
            : 0,
        otherMemPct =
          totalMem > 0
            ? Math.max(0, Math.min((otherMem / totalMem) * 100, 100))
            : 0;
      S.mini.cpu = cpuPct;
      S.mini.mem = memPct;
      miniUpdate();
      E.dashSystemCpu.textContent = tx("dashboard.cpu_line")
        .replace("{cores}", sys.cpu_cores || 0)
        .replace("{goos}", sys.goos || "-")
        .replace("{goarch}", sys.goarch || "-");
      E.dashSystemLoad.textContent = `${tx("dashboard.share.host").replace("{pct}", pct(cpuPct))} | ${tx("dashboard.share.panel").replace("{pct}", pct(panelCpuPct))}`;
      E.dashSystemCPUModel.textContent =
        (sys.cpu_model || tx("dashboard.cpu_model_missing")) +
        " | " +
        tx("dashboard.load_line")
          .replace("{l1}", sys.load_1 || 0)
          .replace("{l5}", sys.load_5 || 0)
          .replace("{l15}", sys.load_15 || 0);
      if (E.dashCpuMain) E.dashCpuMain.textContent = pct(cpuPct);
      if (E.dashCpuSub) E.dashCpuSub.textContent = tx("dashboard.card.cpu");
      ring(E.dashCpuDonut, cpuPct, "#67d4ff", panelCpuPct, "#82ffc8");
      E.dashSystemMem.textContent = `${bstr(usedMem)} / ${bstr(totalMem)}`;
      E.dashAppRSS.textContent = `${tx("dashboard.share.host").replace("{pct}", pct(memPct))} | ${tx("dashboard.share.panel").replace("{pct}", pct(panelMemPct))}`;
      if (E.dashMemMain) E.dashMemMain.textContent = pct(memPct);
      if (E.dashMemSub) E.dashMemSub.textContent = tx("dashboard.card.memory");
      if (E.dashMemMeta)
        E.dashMemMeta.textContent =
          swapTotal > 0
            ? tx("dashboard.swap.line")
                .replace("{used}", bstr(swapUsed))
                .replace("{total}", bstr(swapTotal))
            : tx("dashboard.swap.none");
      ring(E.dashMemDonut, memPct, "#b89aff", panelMemPct, "#67d4ff");
      if (E.dashDiskMeta1)
        E.dashDiskMeta1.textContent = tx("dashboard.disk_mount").replace(
          "{mount}",
          sys.disk_mount || "/",
        );
      if (E.dashDiskMeta2)
        E.dashDiskMeta2.textContent = `${tx("dashboard.share.host").replace("{pct}", pct(diskPct))} | ${tx("dashboard.share.panel").replace("{pct}", pct(panelDiskPct))}`;
      E.dashAppData.textContent = `${tx("dashboard.disk_free").replace("{free}", bstr(diskFree))} | ${tx(
        "dashboard.data_line",
      )
        .replace("{size}", bstr(app.data_size_bytes || 0))
        .replace("{count}", app.running_service_count || 0)}`;
      if (E.dashDiskMain) E.dashDiskMain.textContent = pct(diskPct);
      if (E.dashDiskSub) E.dashDiskSub.textContent = tx("dashboard.card.disk");
      ring(E.dashDiskDonut, diskPct, "#67f0b1", panelDiskPct, "#67d4ff");
    } catch (_) {
      [
        "dashSystemCpu",
        "dashSystemLoad",
        "dashSystemCPUModel",
        "dashSystemMem",
        "dashAppRSS",
        "dashAppData",
        "dashCpuMain",
        "dashCpuSub",
        "dashMemMain",
        "dashMemSub",
        "dashMemMeta",
        "dashDiskMain",
        "dashDiskSub",
        "dashDiskMeta1",
        "dashDiskMeta2",
      ].forEach((id) => {
        if (E[id]) E[id].textContent = "-";
      });
    }
  }
  function groups() {
    const m = { Sealdice: [], LuckyLilliaBot: [], Lagrange: [] };
    S.svcs.forEach((s) => (m[s.type] || (m[s.type] = [])).push(s));
    Object.values(m).forEach((a) =>
      a.sort((x, y) => disp(x).localeCompare(disp(y), "zh-CN")),
    );
    return m;
  }
  function tree() {
    const cMap = {
      Sealdice: "tree-category-seal",
      Lagrange: "tree-category-lagrange",
      LuckyLilliaBot: "tree-category-llbot",
    };
    const g = groups();
    E.serviceTree.innerHTML = Object.entries(g)
      .map(
        ([t, a]) =>
          `<div class="tree-category ${cMap[t] || ""}"><button class="tree-category-btn" type="button" data-gt="${esc(t)}"><span>${esc(typ(t))}</span><span class="tree-count">${a.length}</span></button>${a.length ? `<div class="tree-item-list ${S.collapsed[t] ? "hidden" : ""}">${a.map((s) => `<button class="tree-item ${S.svc && S.svc.id === s.id ? "active" : ""}" type="button" data-sv="${esc(s.id)}"><span>${esc(disp(s))}</span><span class="tree-item-state ${s.status === "running" ? "tree-ok" : "tree-stop"}">${st(s.status)}</span></button>`).join("")}</div>` : `<div class="tree-empty">${tx("tree.empty")}</div>`}</div>`,
      )
      .join("");
    $$("[data-gt]", E.serviceTree).forEach(
      (b) =>
        (b.onclick = () => {
          S.collapsed[b.dataset.gt] = !S.collapsed[b.dataset.gt];
          tree();
        }),
    );
    $$("[data-sv]", E.serviceTree).forEach(
      (b) =>
        (b.onclick = () => {
          view("services");
          openSvc(b.dataset.sv);
        }),
    );
  }
  function filtered() {
    const q = E.serviceSearch.value.trim().toLowerCase(),
      t = E.serviceTypeFilter.value,
      stt = E.serviceStatusFilter.value,
      so = E.serviceSort.value;
    const a = S.svcs.filter(
      (s) =>
        (t === "all" || s.type === t) &&
        (stt === "all" || s.status === stt) &&
        (!q ||
          [s.id, s.name, s.display_name, s.type].some((v) =>
            String(v || "")
              .toLowerCase()
              .includes(q),
          )),
    );
    a.sort((x, y) =>
      so === "type"
        ? typ(x.type).localeCompare(typ(y.type), "zh-CN")
        : so === "status"
          ? st(x.status).localeCompare(st(y.status), "zh-CN")
          : so === "updated"
            ? String(y.updated_at || "").localeCompare(
                String(x.updated_at || ""),
              )
            : disp(x).localeCompare(disp(y), "zh-CN"),
    );
    return a;
  }
  function cards() {
    const a = filtered();
    const typeClass = {
      Sealdice: "svc-theme-seal",
      Lagrange: "svc-theme-lagrange",
      LuckyLilliaBot: "svc-theme-llbot",
    };
    const iconMap = {
      Sealdice: "icons/sealdice.png",
      Lagrange: "icons/lagrange.png",
      LuckyLilliaBot: "icons/llbot.png",
    };
    const groups = [
      ["Sealdice", "service-group-seal"],
      ["LuckyLilliaBot", "service-group-llbot"],
      ["Lagrange", "service-group-lagrange"],
    ];
    const card = (s) =>
      `<button class="service-card ${typeClass[s.type] || ""} ${s.status === "running" ? "svc-running" : "svc-stopped"}" type="button" data-open="${esc(s.id)}"><div class="service-card-head"><div class="service-icon-chip"><img class="service-icon" src="${esc(iconMap[s.type] || "")}" alt="${esc(s.type)}"></div><div class="service-title-block"><strong class="service-title">${esc(disp(s))}</strong><div class="service-reg mono">${esc(s.id)}</div></div><span class="status-chip ${s.status === "running" ? "status-running" : "status-stopped"}">${st(s.status)}</span></div><div class="service-meta-grid service-meta-grid-3"><div class="service-meta-box"><span>${tx("service.meta.type")}</span><strong>${esc(typ(s.type))}</strong></div><div class="service-meta-box"><span>${tx("service.pid")}</span><strong>${s.pid || "-"}</strong></div><div class="service-meta-box"><span>${tx("service.port")}</span><strong>${s.port || "-"}</strong></div><div class="service-meta-box"><span>${tx("service.meta.auto_start")}</span><strong>${s.auto_start ? tx("common.yes") : tx("common.no")}</strong></div><div class="service-meta-box"><span>${tx("service.meta.auto_restart")}</span><strong>${s.restart && s.restart.enabled ? tx("common.yes") : tx("common.no")}</strong></div><div class="service-meta-box"><span>${tx("service.meta.crash_count")}</span><strong>${(s.restart && s.restart.consecutive_crash) || 0}</strong></div></div><div class="service-card-actions"><span class="btn btn-soft">${tx("service.enter_manage")}</span></div></button>`;
    if (!a.length) {
      show(E.servicesEmpty);
      E.servicesEmpty.textContent = tx("service.empty.filtered");
      E.serviceCards.innerHTML = "";
      return;
    }
    hide(E.servicesEmpty);
    E.serviceCards.innerHTML = groups
      .map(([type, cls]) => {
        const items = a.filter((s) => s.type === type);
        return `<section class="service-type-column ${cls}"><div class="service-type-head"><strong>${esc(typ(type))}</strong><span class="service-type-count">${items.length}</span></div><div class="service-type-body">${items.length ? items.map(card).join("") : `<div class="service-type-empty">${tx("tree.empty")}</div>`}</div></section>`;
      })
      .join("");
    $$("[data-open]", E.serviceCards).forEach(
      (b) => (b.onclick = () => openSvc(b.dataset.open)),
    );
  }
  async function loadSvcs(silent = false) {
    try {
      const d = await j("services");
      S.svcs = d.services || [];
      dash();
      dashMetrics();
      tree();
      cards();
      miniUpdate();
      if (S.svc) {
        const f = S.svcs.find((x) => x.id === S.svc.id);
        if (f) {
          S.svc = f;
          renderSvc();
        } else closeSvc();
      }
      if (!silent)
        E.statusBar.textContent = tx("service.action.loaded").replace(
          "{count}",
          S.svcs.length,
        );
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function renderSvc() {
    const s = S.svc;
    if (!s) return;
    hide(E.servicesListView);
    show(E.serviceDetailView);
    E.detailBreadcrumb.textContent = tx("detail.breadcrumb.current")
      .replace("{type}", typ(s.type))
      .replace("{id}", s.id);
    E.detailName.textContent = disp(s);
    E.detailMeta.textContent = tx("detail.meta.compact")
      .replace("{id}", s.id)
      .replace("{type}", typ(s.type))
      .replace("{time}", tstr(s.created_at));
    E.detailStatusValue.textContent = st(s.status);
    E.detailPortValue.textContent = s.port || "-";
    E.detailPidValue.textContent = s.pid || "-";
    if (E.detailStatusPill) {
      E.detailStatusPill.classList.remove("state-running", "state-stopped");
      E.detailStatusPill.classList.add(
        s.status === "running" ? "state-running" : "state-stopped",
      );
    }
    E.overviewPathValue.textContent = s.install_dir || s.work_dir || "-";
    E.overviewTypeValue.textContent = typ(s.type);
    E.detailForceBtn.textContent = tx("detail.force_stop");
    E.detailEntryBtn.textContent =
      s.type === "Lagrange" ? tx("detail.entry.qr") : tx("detail.entry.open");
    E.detailConfigBtn.textContent = tx("detail.config_center");
    if (s.type === "LuckyLilliaBot") {
      E.llbotNotice.textContent = LLN();
      show(E.llbotNotice);
    } else hide(E.llbotNotice);
    more();
    tab(S.tab);
    tree();
    logLoad();
    svcMetrics();
    const u = new URL(window.location.href);
    u.searchParams.set("service", s.id);
    history.replaceState(
      null,
      "",
      u.pathname + "?" + u.searchParams.toString() + u.hash,
    );
  }
  async function openSvc(id) {
    const prev = S.svc ? S.svc.id : "";
    const s = S.svcs.find((x) => x.id === id);
    if (!s) {
      await loadSvcs(true);
    }
    S.svc = S.svcs.find((x) => x.id === id);
    if (!S.svc) {
      toast(tx("toast.service_missing"), "error");
      return;
    }
    if (prev && prev !== id) {
      S.file.paths[prev] = S.file.path || "";
      S.file.parent = "";
      S.file.entries = [];
      S.file.sel = new Set();
    }
    S.file.path = S.file.paths[id] || "";
    renderSvc();
  }
  async function svcMetrics() {
    if (!S.svc) return;
    try {
      const d = await j(`services/${encodeURIComponent(S.svc.id)}/metrics`);
      E.overviewRssValue.textContent = bstr(d.rss_bytes || 0);
      E.overviewInstallSizeValue.textContent = bstr(d.install_size_bytes || 0);
      E.overviewLogSizeValue.textContent = bstr(d.log_size_bytes || 0);
    } catch (_) {
      E.overviewRssValue.textContent = "-";
      E.overviewInstallSizeValue.textContent = "-";
      E.overviewLogSizeValue.textContent = "-";
    }
  }
  function closeSvc() {
    S.svc = null;
    logStop();
    show(E.servicesListView);
    hide(E.serviceDetailView);
    const t = appPath("/services");
    history.replaceState(null, "", t);
    tree();
  }
  function tab(name) {
    S.tab = name;
    ["overview", "logs", "files"].forEach((n) => {
      const p = $("tab" + n[0].toUpperCase() + n.slice(1));
      if (!p) return;
      n === name ? show(p) : hide(p);
    });
    $$(".tab-btn").forEach((b) =>
      b.classList.toggle("active", b.dataset.tab === name),
    );
    if (name === "files") files();
    if (name === "overview" || name === "logs") logStart();
  }
  function more() {
    const s = S.svc;
    if (!s) return;
    const a = [
      ["rebuild", tx("detail.rebuild"), "btn-rebuild btn-menu-mini"],
      ["delete", tx("detail.delete"), "btn-danger btn-menu-mini"],
    ];
    E.detailMoreMenu.innerHTML = a
      .map(
        ([k, l, c]) =>
          `<button class="btn ${c}" type="button" data-ma="${k}">${l}</button>`,
      )
      .join("");
    $$("[data-ma]", E.detailMoreMenu).forEach(
      (b) =>
        (b.onclick = async () => {
          hide(E.detailMoreMenu);
          const a = b.dataset.ma;
          if (a === "rebuild") {
            await rebuild();
          } else if (a === "delete") delOpen();
        }),
    );
  }
  async function svcAct(a) {
    if (!S.svc) return;
    try {
      await j(`services/${encodeURIComponent(S.svc.id)}/${a}`, {
        method: "POST",
      });
      await loadSvcs(true);
      toast(tx("toast.done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function logStart() {
    logStop();
    S.logs.timer = setInterval(logLoad, 1800);
  }
  function logStop() {
    if (S.logs.timer) {
      clearInterval(S.logs.timer);
      S.logs.timer = 0;
    }
  }
  function logRangeText() {
    const startAt = S.svc
      ? S.svc.status === "running"
        ? tstr(S.svc.last_start_at || S.svc.updated_at || S.svc.created_at)
        : "-"
      : "-";
    const refreshed = tstr(new Date().toISOString());
    return tx("detail.logs.range")
      .replace("{from}", startAt)
      .replace("{to}", refreshed);
  }
  async function logLoad() {
    if (!S.svc || S.logs.paused) return;
    try {
      const d = await j(
        `services/${encodeURIComponent(S.svc.id)}/logs?lines=500`,
      );
      S.logs.all = d.content || "";
      const q = E.logSearch.value.trim().toLowerCase();
      const t = q
        ? S.logs.all
            .split(/\r?\n/)
            .filter((x) => x.toLowerCase().includes(q))
            .join("\n")
        : S.logs.all;
      setPre(E.logsBox, t);
      setPre(
        E.overviewLogPreview,
        S.logs.all.split(/\r?\n/).slice(-40).join("\n"),
      );
      if (E.logsRange) E.logsRange.textContent = logRangeText();
    } catch (e) {
      if (e.status !== 401) {
        setPre(
          E.logsBox,
          tx("detail.logs.read_failed").replace("{msg}", e.message),
        );
        if (E.logsRange)
          E.logsRange.textContent = tx("detail.logs.range")
            .replace("{from}", "-")
            .replace("{to}", "-");
      }
    }
  }
  async function logHistoryLoad() {
    if (!S.svc) return;
    const d = await j(`services/${encodeURIComponent(S.svc.id)}/logs/history`);
    S.logHistory.items = Array.isArray(d.items) ? d.items : [];
    const rows = S.logHistory.items
      .map((it) => {
        const rawName = String(it.name || "-");
        const name = encodeURIComponent(rawName);
        const start = tstr(it.created_at || "").replace("T", " ");
        const ended = tstr(it.ended_at || "").replace("T", " ");
        const tm = ended && ended !== "-" ? `${start} -> ${ended}` : start;
        const sz = bstr(Number(it.size || 0));
        return `<tr data-log-item="${name}"><td>${esc(tm)}</td><td>${esc(sz)}</td><td><div class="inline-tools"><button class="btn btn-soft btn-history-inline" type="button" data-log-download="${name}">${tx("file.download")}</button><button class="btn btn-soft btn-history-inline btn-danger-outline" type="button" data-log-delete="${name}">${tx("file.delete")}</button></div></td></tr>`;
      })
      .join("");
    E.logHistoryBody.innerHTML =
      rows || `<tr><td colspan="3" class="muted">${tx("common.no_data")}</td></tr>`;
    $$("[data-log-item]", E.logHistoryBody).forEach((row) => {
      row.onclick = () =>
        logHistoryOpenItem(decodeURIComponent(row.dataset.logItem || ""));
    });
    $$("[data-log-download]", E.logHistoryBody).forEach((btn) => {
      btn.onclick = (e) => {
        e.stopPropagation();
        downloadHistoryItem(decodeURIComponent(btn.dataset.logDownload || ""));
      };
    });
    $$("[data-log-delete]", E.logHistoryBody).forEach((btn) => {
      btn.onclick = async (e) => {
        e.stopPropagation();
        await deleteHistoryItem(decodeURIComponent(btn.dataset.logDelete || ""));
      };
    });
  }
  async function logHistoryOpenItem(name) {
    if (!S.svc || !name) return;
    const d = await j(
      `services/${encodeURIComponent(S.svc.id)}/logs/history/${encodeURIComponent(name)}?lines=500`,
    );
    S.logHistory.name = d.name || name;
    S.logHistory.content = d.content || "";
    E.logHistoryCurrentName.textContent = S.logHistory.name;
    renderLogHistoryContent();
  }
  function renderLogHistoryContent() {
    const kw = String((E.logHistorySearch && E.logHistorySearch.value) || "")
      .trim()
      .toLowerCase();
    if (!kw) {
      setPre(E.logHistoryContent, S.logHistory.content || "");
      return;
    }
    const filtered = String(S.logHistory.content || "")
      .split(/\r?\n/)
      .filter((line) => line.toLowerCase().includes(kw))
      .join("\n");
    setPre(E.logHistoryContent, filtered);
  }
  async function openLogHistory() {
    if (!S.svc) return;
    try {
      await logHistoryLoad();
      if (E.logHistorySearch) E.logHistorySearch.value = "";
      if (S.logHistory.items.length) {
        await logHistoryOpenItem(S.logHistory.items[0].name);
      } else {
        E.logHistoryCurrentName.textContent = "-";
        setPre(E.logHistoryContent, "");
      }
      show(E.logHistoryModal);
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function downloadHistoryItem(name) {
    if (!name || !S.svc) return;
    let content = S.logHistory.name === name ? S.logHistory.content : "";
    if (!content) {
      const d = await j(
        `services/${encodeURIComponent(S.svc.id)}/logs/history/${encodeURIComponent(name)}?lines=1000000`,
      );
      content = d.content || "";
    }
    const b = new Blob([content], {
      type: "text/plain;charset=utf-8",
    });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(b);
    a.download = `${S.svc ? S.svc.id : "service"}-${name}.log`;
    a.click();
    URL.revokeObjectURL(a.href);
  }
  async function deleteHistoryItem(name) {
    if (!S.svc || !name) {
      return;
    }
    if (!(await askConfirm(tx("confirm.delete_history_log").replace("{name}", name))))
      return;
    try {
      await j(
        `services/${encodeURIComponent(S.svc.id)}/logs/history/${encodeURIComponent(name)}`,
        { method: "DELETE" },
      );
      await logHistoryLoad();
      if (S.logHistory.items.length) {
        await logHistoryOpenItem(S.logHistory.items[0].name);
      } else {
        S.logHistory.name = "";
        S.logHistory.content = "";
        E.logHistoryCurrentName.textContent = "-";
        setPre(E.logHistoryContent, "");
      }
      toast(tx("msg.delete_done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function clearHistoryLogs() {
    if (!S.svc) return;
    if (!(await askConfirm(tx("confirm.clear_history_logs")))) return;
    try {
      await j(`services/${encodeURIComponent(S.svc.id)}/logs/history`, {
        method: "DELETE",
      });
      S.logHistory.name = "";
      S.logHistory.content = "";
      E.logHistoryCurrentName.textContent = "-";
      setPre(E.logHistoryContent, "");
      await logHistoryLoad();
      toast(tx("msg.delete_done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function isTxt(n) {
    n = String(n || "").toLowerCase();
    return TXT.some((x) => n.endsWith(x));
  }
  function isArc(n) {
    n = String(n || "").toLowerCase();
    return ARC.some((x) => n.endsWith(x));
  }
  function picked() {
    return S.file.entries.filter((e) => S.file.sel.has(e.path));
  }
  function fInfo() {
    const a = picked();
    if (!a.length) {
      E.fileInfoBody.textContent = tx("file.none_selected");
      return;
    }
    if (a.length === 1) {
      const x = a[0];
      E.fileInfoBody.textContent = tx("file.info.single")
        .replace("{name}", x.name)
        .replace(
          "{type}",
          x.is_dir ? tx("file.type.dir") : x.type || tx("file.type.file"),
        )
        .replace("{size}", x.is_dir ? "-" : x.size || 0)
        .replace("{path}", x.path);
      return;
    }
    E.fileInfoBody.textContent = tx("file.info.multi").replace(
      "{count}",
      a.length,
    );
  }
  async function files(path = S.file.path || "") {
    if (!S.svc) return;
    try {
      show(E.filesLockedNotice);
      E.filesLockedText.textContent = tx("file.warning.live_edit");
      S.file.path = path || "";
      const d = await j(
        `services/${encodeURIComponent(S.svc.id)}/files?path=${encodeURIComponent(path)}`,
      );
      S.file.path = d.current_path || "";
      S.file.paths[S.svc.id] = S.file.path;
      S.file.parent = d.parent_path || "";
      S.file.entries = d.entries || [];
      if (!S.file.entries.some((entry) => entry.path === S.file.focus)) {
        S.file.focus = "";
      }
      S.file.sel = new Set();
      fRender();
    } catch (e) {
      if (S.file.path) S.file.path = "";
      toast(e.message, "error");
    }
  }
  function fRender() {
    const q = E.fileSearch.value.trim().toLowerCase();
    const s = E.fileSort.value;
    let a = S.file.entries.filter(
      (e) =>
        !q ||
        String(e.name || "")
          .toLowerCase()
          .includes(q),
    );
    a = a
      .slice()
      .sort((x, y) =>
        x.is_dir !== y.is_dir
          ? x.is_dir
            ? -1
            : 1
          : s === "size"
            ? (x.size || 0) - (y.size || 0)
            : s === "time"
              ? String(y.mod_time || "").localeCompare(String(x.mod_time || ""))
              : s === "type"
                ? String(x.type || "").localeCompare(
                    String(y.type || ""),
                    "zh-CN",
                  )
                : String(x.name || "").localeCompare(
                    String(y.name || ""),
                    "zh-CN",
                  ),
      );
    E.fileBreadcrumb.textContent = "/" + (S.file.path || "");
    E.fileTableBody.innerHTML = a
      .map(
        (e) =>
          `<tr class="file-row ${S.file.focus === e.path ? "file-row-active" : ""} ${S.file.sel.has(e.path) ? "file-row-selected" : ""}" data-fr="${esc(e.path)}"><td><input type="checkbox" data-fc="${esc(e.path)}" ${S.file.sel.has(e.path) ? "checked" : ""}></td><td>${e.is_dir ? "&#128193; " : "&#128196; "}${esc(e.name)}</td><td>${e.is_dir ? tx("file.type.dir") : esc(e.type || tx("file.type.file"))}</td><td>${e.is_dir ? "-" : e.size || 0}</td><td>${esc(tstr(e.mod_time))}</td></tr>`,
      )
      .join("");
    $$("[data-fc]", E.fileTableBody).forEach(
      (i) =>
        (i.onchange = () => {
          i.checked
            ? S.file.sel.add(i.dataset.fc)
            : S.file.sel.delete(i.dataset.fc);
          const row = i.closest("[data-fr]");
          if (row) row.classList.toggle("file-row-selected", i.checked);
          fInfo();
        }),
    );
    $$("[data-fr]", E.fileTableBody).forEach((r) => {
      r.onclick = (e) => {
        if (e.target && e.target.matches("input")) return;
        const currentPath = r.dataset.fr || "";
        S.file.focus = currentPath;
        $$("[data-fr]", E.fileTableBody).forEach((row) => {
          row.classList.toggle("file-row-active", row.dataset.fr === currentPath);
        });
      };
      r.ondblclick = async () => {
        const x = S.file.entries.find((v) => v.path === r.dataset.fr);
        if (!x) return;
        if (x.is_dir) {
          await files(x.path);
          return;
        }
        if (isTxt(x.name)) {
          await edOpen(x.path);
          return;
        }
        if (isArc(x.name) && (await askConfirm(tx("file.extract.ask")))) {
          await fPost("extract", { path: x.path, destination: S.file.path });
        }
      };
    });
    fInfo();
  }
  async function fPost(a, b, opt = {}) {
    if (!S.svc) return;
    try {
      if (opt.form) {
        await j(
          `services/${encodeURIComponent(S.svc.id)}/files/${a}${opt.q ? "?" + opt.q : ""}`,
          { method: "POST", body: opt.form },
        );
      } else
        await j(`services/${encodeURIComponent(S.svc.id)}/files/${a}`, {
          method: "POST",
          body: JSON.stringify(b),
        });
      await files(S.file.path);
      toast(tx("msg.saved"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function monacoLang(file) {
    const ext = String(file || "")
      .toLowerCase()
      .split(".")
      .pop();
    const map = {
      js: "javascript",
      ts: "typescript",
      json: "json",
      css: "css",
      html: "html",
      md: "markdown",
      xml: "xml",
      yaml: "yaml",
      yml: "yaml",
      sh: "shell",
      ini: "ini",
    };
    return map[ext] || "plaintext";
  }
  async function loadMonaco() {
    if (window.monaco && S.editor.instance) return window.monaco;
    if (S.editor.loader) return S.editor.loader;
    S.editor.loader = new Promise((resolve, reject) => {
      const finish = () => {
        window.require.config({ paths: { vs: appPath("/vendor/monaco/vs") } });
        window.require(
          ["vs/editor/editor.main"],
          () => {
            const editor = window.monaco.editor.create(
              E.editorMount,
              {
                value: "",
                language: "plaintext",
                theme: "vs-dark",
                automaticLayout: true,
                minimap: { enabled: false },
                fontSize: 14,
                scrollBeyondLastLine: false,
              },
              {},
            );
            editor.onDidChangeModelContent(() => {
              S.editor.dirty = editor.getValue() !== S.editor.saved;
              edMeta();
            });
            editor.onDidChangeCursorPosition(() => edMeta());
            S.editor.instance = editor;
            resolve(window.monaco);
          },
          reject,
        );
      };
      if (window.require && window.require.config) {
        finish();
        return;
      }
      const script = document.createElement("script");
      script.src = appPath("/vendor/monaco/vs/loader.js");
      script.onload = finish;
      script.onerror = () => reject(new Error(tx("editor.load_failed")));
      document.head.appendChild(script);
    });
    return S.editor.loader;
  }
  function edValue() {
    return S.editor.instance
      ? S.editor.instance.getValue()
      : E.editorText.value;
  }
  async function edOpen(path) {
    try {
      const d = await j(
        `services/${encodeURIComponent(S.svc.id)}/files/text?path=${encodeURIComponent(path)}`,
      );
      S.editor.path = d.path || path;
      S.editor.saved = d.content || "";
      S.editor.dirty = false;
      E.editorTitle.textContent = tx("editor.edit_prefix") + S.editor.path;
      try {
        await loadMonaco();
        hide(E.editorText);
        if (S.editor.instance) {
          S.editor.instance.setValue(S.editor.saved);
          window.monaco.editor.setModelLanguage(
            S.editor.instance.getModel(),
            monacoLang(S.editor.path),
          );
        }
      } catch (_) {
        show(E.editorText);
        E.editorText.value = S.editor.saved;
      }
      edMeta();
      show(E.editorModal);
      if (S.editor.instance) S.editor.instance.focus();
      else E.editorText.focus();
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function edMeta() {
    const ext = S.editor.path.includes(".")
      ? S.editor.path.slice(S.editor.path.lastIndexOf("."))
      : tx("editor.lang.unknown");
    E.editorLang.textContent = ext;
    E.editorState.textContent = S.editor.dirty
      ? tx("editor.state.unsaved")
      : tx("editor.state.saved");
    if (S.editor.instance) {
      const pos = S.editor.instance.getPosition() || {
        lineNumber: 1,
        column: 1,
      };
      E.editorCursor.textContent = `Ln ${pos.lineNumber}, Col ${pos.column}`;
      return;
    }
    const p0 = E.editorText.selectionStart || 0;
    const ls = E.editorText.value.slice(0, p0).split("\n");
    E.editorCursor.textContent = `Ln ${ls.length}, Col ${ls[ls.length - 1].length + 1}`;
  }
  async function edSave() {
    if (!S.svc || !S.editor.path) return;
    try {
      const content = edValue();
      await j(`services/${encodeURIComponent(S.svc.id)}/files/text`, {
        method: "POST",
        body: JSON.stringify({ path: S.editor.path, content }),
      });
      S.editor.saved = content;
      S.editor.dirty = false;
      edMeta();
      await files(S.file.path);
      toast(tx("msg.file_saved"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function edClose() {
    if (S.editor.dirty && !(await askConfirm(tx("editor.unsaved_confirm")))) return;
    hide(E.editorModal);
  }

  /**
   * 配置中心委托包装函数。
   * 保留这些同名入口，是为了不破坏已有事件绑定与调用路径；
   * 真正的配置中心逻辑在 app-settings-center.js 中实现。
   */
  async function setOpen() {
    if (settingsCenterApi && typeof settingsCenterApi.setOpen === "function") {
      return settingsCenterApi.setOpen();
    }
    toast("settings-center not ready", "error");
  }

  async function setSave() {
    if (settingsCenterApi && typeof settingsCenterApi.setSave === "function") {
      return settingsCenterApi.setSave();
    }
    toast("settings-center not ready", "error");
  }

  async function portsOpen() {
    if (
      settingsCenterApi &&
      typeof settingsCenterApi.portsOpen === "function"
    ) {
      return settingsCenterApi.portsOpen();
    }
    return setOpen();
  }

  async function portsSave() {
    if (
      settingsCenterApi &&
      typeof settingsCenterApi.portsSave === "function"
    ) {
      return settingsCenterApi.portsSave();
    }
    return setSave();
  }

  async function panelLoad() {
    try {
      const d = await j("panel/settings");
      const c = d.config || {};
      E.panelListenHost.value = c.listen_host || "0.0.0.0";
      E.panelListenPort.value = c.listen_port || 3210;
      E.panelBasePath.value = c.base_path || "/";
      E.panelSessionTTL.value = c.session_ttl_hours || 48;
      if (E.panelTrustProxy)
        E.panelTrustProxy.checked = !!c.trust_proxy_headers;
      if (E.panelDisableHTTPSWarn)
        E.panelDisableHTTPSWarn.checked = !!c.disable_https_warning;
      if (E.panelLogRetention)
        E.panelLogRetention.value = c.log_retention_count || 10;
      if (E.panelLogRetentionDays)
        E.panelLogRetentionDays.value = c.log_retention_days || 30;
      if (E.panelLogMaxMB) E.panelLogMaxMB.value = c.log_max_mb || 2048;
      if (E.panelMetricsRefresh)
        E.panelMetricsRefresh.value = c.metrics_refresh_seconds || 2;
      const panelFileManagerEnabled = $("panelFileManagerEnabled");
      const panelFileUploadMaxMB = $("panelFileUploadMaxMB");
      const panelLoginProtectEnabled = $("panelLoginProtectEnabled");
      const panelLoginProtectMaxAttempts = $("panelLoginProtectMaxAttempts");
      const panelLoginProtectWindow = $("panelLoginProtectWindow");
      const panelLoginProtectBlock = $("panelLoginProtectBlock");
      const panelLoginProtectMaxBuckets = $("panelLoginProtectMaxBuckets");
      const panelLoginProtectBucketIdleTTL = $("panelLoginProtectBucketIdleTTL");
      const panelLoginProtectCleanupInterval = $("panelLoginProtectCleanupInterval");
      const panelSessionMaxEntries = $("panelSessionMaxEntries");
      const panelSessionCleanupInterval = $("panelSessionCleanupInterval");
      if (panelFileManagerEnabled)
        panelFileManagerEnabled.checked = !!(
          c.file_manager && c.file_manager.enabled
        );
      if (panelFileUploadMaxMB)
        panelFileUploadMaxMB.value =
          (c.file_manager && c.file_manager.upload_max_mb) || 2048;
      if (panelLoginProtectEnabled)
        panelLoginProtectEnabled.checked = !!(
          c.login_protect && c.login_protect.enabled
        );
      if (panelLoginProtectMaxAttempts)
        panelLoginProtectMaxAttempts.value =
          (c.login_protect && c.login_protect.max_attempts) || 20;
      if (panelLoginProtectWindow)
        panelLoginProtectWindow.value =
          (c.login_protect && c.login_protect.window_seconds) || 600;
      if (panelLoginProtectBlock)
        panelLoginProtectBlock.value =
          (c.login_protect && c.login_protect.block_seconds) || 600;
      if (panelLoginProtectMaxBuckets)
        panelLoginProtectMaxBuckets.value =
          (c.login_protect && c.login_protect.max_buckets) || 10000;
      if (panelLoginProtectBucketIdleTTL)
        panelLoginProtectBucketIdleTTL.value =
          (c.login_protect && c.login_protect.bucket_idle_ttl_seconds) || 3600;
      if (panelLoginProtectCleanupInterval)
        panelLoginProtectCleanupInterval.value =
          (c.login_protect && c.login_protect.cleanup_interval_seconds) || 300;
      if (panelSessionMaxEntries)
        panelSessionMaxEntries.value = c.session_max_entries || 10000;
      if (panelSessionCleanupInterval)
        panelSessionCleanupInterval.value = c.session_cleanup_interval_seconds || 300;
      E.panelRawConfig.value = d.raw || "";
      E.panelRawMeta.textContent = tx("panel.settings.raw_meta").replace(
        "{path}",
        d.config_path || "config.yaml",
      );
      if (E.panelUpdateStatus)
        E.panelUpdateStatus.textContent = TXT_SAFE(
          "panel.update.idle",
          "点击“检查更新”获取最新版本信息。",
        );
      if (E.panelUpdateApplyBtn) {
        E.panelUpdateApplyBtn.disabled = true;
      }
      S.disableHTTPSWarning = !!c.disable_https_warning;
      updateHTTPSNotice();
    } catch (e) {
      toast(e.message, "error");
    }
  }

  function renderUpdateCheckResult(d) {
    if (!E.panelUpdateStatus) return;
    const local = String(d.local_version || "-");
    const remote = String(d.remote_version || "-");
    const source = String(d.manifest_source || d.manifest_source_id || "-");
    const hasUpdate = !!d.has_update;
    S.update.checked = true;
    S.update.hasUpdate = hasUpdate;
    S.update.result = d || null;
    if (E.shellVersion) E.shellVersion.classList.toggle("has-update", hasUpdate);
    if (E.panelUpdateApplyBtn) E.panelUpdateApplyBtn.disabled = !hasUpdate;
    const statusText = hasUpdate
      ? TXT_SAFE("panel.update.available", "Update available")
      : TXT_SAFE("panel.update.latest", "Already latest");
    E.panelUpdateStatus.textContent =
      `${statusText}
` +
      `${TXT_SAFE("panel.update.local", "Local")}: ${local}
` +
      `${TXT_SAFE("panel.update.remote", "Remote")}: ${remote}
` +
      `${TXT_SAFE("panel.update.source", "Source")}: ${source}`;
  }

  async function panelCheckUpdate() {
    if (!E.panelUpdateCheckBtn) return;
    try {
      E.panelUpdateCheckBtn.disabled = true;
      if (E.panelUpdateStatus)
        E.panelUpdateStatus.textContent = TXT_SAFE(
          "panel.update.checking",
          "Checking...",
        );
      const d = await j("update/check");
      renderUpdateCheckResult(d || {});
      if (d && d.has_update) {
        toast(TXT_SAFE("panel.update.toast.available", "Update available"), "ok");
      } else {
        toast(TXT_SAFE("panel.update.toast.latest", "Already latest"), "ok");
      }
    } catch (e) {
      if (E.panelUpdateStatus)
        E.panelUpdateStatus.textContent = TXT_SAFE(
          "panel.update.failed",
          "Check update failed",
        );
      toast(e.message || TXT_SAFE("panel.update.failed", "Check update failed"), "error");
    } finally {
      E.panelUpdateCheckBtn.disabled = false;
    }
  }

  async function panelApplyUpdate() {
    if (!E.panelUpdateApplyBtn) return;
    try {
      if (!S.update.checked) await panelCheckUpdate();
      if (!S.update.hasUpdate) {
        toast(TXT_SAFE("panel.update.latest", "Already latest"), "info");
        return;
      }
      const ok = await askConfirm(
        TXT_SAFE(
          "panel.update.confirm",
          "Apply update and restart panel/services now? If failed, run installer script manually.",
        ),
      );
      if (!ok) return;
      E.panelUpdateApplyBtn.disabled = true;
      if (E.panelUpdateStatus)
        E.panelUpdateStatus.textContent = TXT_SAFE(
          "panel.update.applying",
          "Applying update...",
        );
      const d = await j("update/apply", { method: "POST" });
      const msg = String((d && d.message) || "");
      toast(msg || TXT_SAFE("panel.update.applied", "Update command sent, restarting."), "ok");
      setTimeout(() => {
        window.location.href = "/";
      }, 2000);
    } catch (e) {
      if (E.panelUpdateApplyBtn) E.panelUpdateApplyBtn.disabled = false;
      if (E.panelUpdateStatus)
        E.panelUpdateStatus.textContent = TXT_SAFE(
          "panel.update.apply_failed",
          "Update failed, please use installer script.",
        );
      toast(
        e.message || TXT_SAFE("panel.update.apply_failed", "Update failed, please use installer script."),
        "error",
      );
    }
  }

  async function panelCheckUpdateSilent() {
    try {
      const d = await j("update/check");
      renderUpdateCheckResult(d || {});
    } catch (_) {
      // silent check on startup
    }
  }
  async function panelSaveForm() {
    try {
      const port = p(E.panelListenPort.value);
      const ttl = Number(E.panelSessionTTL.value || 0),
        retention = Number(E.panelLogRetention.value || 0),
        retentionDays = Number((E.panelLogRetentionDays && E.panelLogRetentionDays.value) || 0),
        logMaxMB = Number((E.panelLogMaxMB && E.panelLogMaxMB.value) || 0),
        refresh = Number(E.panelMetricsRefresh.value || 0);
      const panelFileManagerEnabled = $("panelFileManagerEnabled");
      const panelFileUploadMaxMB = $("panelFileUploadMaxMB");
      const panelLoginProtectEnabled = $("panelLoginProtectEnabled");
      const panelLoginProtectMaxAttempts = $("panelLoginProtectMaxAttempts");
      const panelLoginProtectWindow = $("panelLoginProtectWindow");
      const panelLoginProtectBlock = $("panelLoginProtectBlock");
      const panelLoginProtectMaxBuckets = $("panelLoginProtectMaxBuckets");
      const panelLoginProtectBucketIdleTTL = $("panelLoginProtectBucketIdleTTL");
      const panelLoginProtectCleanupInterval = $("panelLoginProtectCleanupInterval");
      const panelSessionMaxEntries = $("panelSessionMaxEntries");
      const panelSessionCleanupInterval = $("panelSessionCleanupInterval");
      const panelDisableHTTPSWarn = $("panelDisableHTTPSWarn");
      const uploadMax = Number(
        (panelFileUploadMaxMB && panelFileUploadMaxMB.value) || 0,
      );
      const protectMax = Number(
        (panelLoginProtectMaxAttempts && panelLoginProtectMaxAttempts.value) ||
          0,
      );
      const protectWindow = Number(
        (panelLoginProtectWindow && panelLoginProtectWindow.value) || 0,
      );
      const protectBlock = Number(
        (panelLoginProtectBlock && panelLoginProtectBlock.value) || 0,
      );
      if (!Number.isInteger(ttl) || ttl < 1)
        throw new Error(tx("panel.settings.invalid_ttl"));
      if (!Number.isInteger(retention) || retention < 1 || retention > 365)
        throw new Error(tx("panel.settings.invalid_log_retention"));
      if (!Number.isInteger(retentionDays) || retentionDays < 1 || retentionDays > 3650)
        throw new Error(TXT_SAFE("panel.settings.invalid_log_retention_days", "日志保留天数无效"));
      if (!Number.isInteger(logMaxMB) || logMaxMB < 16 || logMaxMB > 1024 * 1024)
        throw new Error(TXT_SAFE("panel.settings.invalid_log_max_mb", "日志最大占用无效"));
      if (!Number.isInteger(refresh) || refresh < 1 || refresh > 3600)
        throw new Error(tx("panel.settings.invalid_metrics_refresh"));
      if (!Number.isInteger(uploadMax) || uploadMax < 1 || uploadMax > 4096)
        throw new Error(tx("panel.settings.invalid_file_upload_max_mb"));
      if (!Number.isInteger(protectMax) || protectMax < 1 || protectMax > 10000)
        throw new Error(
          tx("panel.settings.invalid_login_protect_max_attempts"),
        );
      if (
        !Number.isInteger(protectWindow) ||
        protectWindow < 1 ||
        protectWindow > 86400
      )
        throw new Error(
          tx("panel.settings.invalid_login_protect_window_seconds"),
        );
      if (
        !Number.isInteger(protectBlock) ||
        protectBlock < 1 ||
        protectBlock > 86400
      )
        throw new Error(
          tx("panel.settings.invalid_login_protect_block_seconds"),
        );
      const protectMaxBuckets = Number(
        panelLoginProtectMaxBuckets && panelLoginProtectMaxBuckets.value,
      );
      const protectBucketIdle = Number(
        panelLoginProtectBucketIdleTTL && panelLoginProtectBucketIdleTTL.value,
      );
      const protectCleanup = Number(
        panelLoginProtectCleanupInterval && panelLoginProtectCleanupInterval.value,
      );
      const sessionMaxEntries = Number(
        panelSessionMaxEntries && panelSessionMaxEntries.value,
      );
      const sessionCleanup = Number(
        panelSessionCleanupInterval && panelSessionCleanupInterval.value,
      );
      if (
        !Number.isInteger(protectMaxBuckets) ||
        protectMaxBuckets < 100 ||
        protectMaxBuckets > 200000
      )
        throw new Error(tx("panel.settings.invalid_login_protect_max_buckets"));
      if (
        !Number.isInteger(protectBucketIdle) ||
        protectBucketIdle < 60 ||
        protectBucketIdle > 604800
      )
        throw new Error(
          tx("panel.settings.invalid_login_protect_bucket_idle_ttl"),
        );
      if (
        !Number.isInteger(protectCleanup) ||
        protectCleanup < 10 ||
        protectCleanup > 3600
      )
        throw new Error(
          tx("panel.settings.invalid_login_protect_cleanup_interval"),
        );
      if (
        !Number.isInteger(sessionMaxEntries) ||
        sessionMaxEntries < 100 ||
        sessionMaxEntries > 200000
      )
        throw new Error(tx("panel.settings.invalid_session_max_entries"));
      if (
        !Number.isInteger(sessionCleanup) ||
        sessionCleanup < 10 ||
        sessionCleanup > 3600
      )
        throw new Error(tx("panel.settings.invalid_session_cleanup_interval"));
      const d = await j("panel/settings", {
        method: "POST",
        body: JSON.stringify({
          mode: "form",
          config: {
            listen_host: E.panelListenHost.value.trim() || "0.0.0.0",
            listen_port: port,
            base_path: E.panelBasePath.value.trim() || "/",
            trust_proxy_headers: !!(
              E.panelTrustProxy && E.panelTrustProxy.checked
            ),
            disable_https_warning: !!(
              panelDisableHTTPSWarn && panelDisableHTTPSWarn.checked
            ),
            log_retention_count: retention,
            log_retention_days: retentionDays,
            log_max_mb: logMaxMB,
            metrics_refresh_seconds: refresh,
            session_ttl_hours: ttl,
            file_manager_enabled: !!(
              panelFileManagerEnabled && panelFileManagerEnabled.checked
            ),
            file_upload_max_mb: uploadMax,
            login_protect_enabled: !!(
              panelLoginProtectEnabled && panelLoginProtectEnabled.checked
            ),
            login_protect_max_attempts: protectMax,
            login_protect_window_seconds: protectWindow,
            login_protect_block_seconds: protectBlock,
            login_protect_max_buckets: protectMaxBuckets,
            login_protect_bucket_idle_ttl_seconds: protectBucketIdle,
            login_protect_cleanup_interval_seconds: protectCleanup,
            session_max_entries: sessionMaxEntries,
            session_cleanup_interval_seconds: sessionCleanup,
          },
        }),
      });
      S.metricsRefreshSeconds = refresh;
      S.disableHTTPSWarning = !!(
        panelDisableHTTPSWarn && panelDisableHTTPSWarn.checked
      );
      dashPolling();
      updateHTTPSNotice();
      E.panelRawConfig.value = d.raw || E.panelRawConfig.value;
      E.panelRawMeta.textContent = tx("panel.settings.raw_meta").replace(
        "{path}",
        d.config_path || "config.yaml",
      );
      toast(tx("panel.settings.saved"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function panelSaveRaw() {
    try {
      const d = await j("panel/settings", {
        method: "POST",
        body: JSON.stringify({ mode: "raw", raw: E.panelRawConfig.value }),
      });
      const c = d.config || {};
      E.panelListenHost.value = c.listen_host || "0.0.0.0";
      E.panelListenPort.value = c.listen_port || 3210;
      E.panelBasePath.value = c.base_path || "/";
      E.panelSessionTTL.value = c.session_ttl_hours || 48;
      if (E.panelTrustProxy)
        E.panelTrustProxy.checked = !!c.trust_proxy_headers;
      if (E.panelDisableHTTPSWarn)
        E.panelDisableHTTPSWarn.checked = !!c.disable_https_warning;
      if (E.panelLogRetention)
        E.panelLogRetention.value = c.log_retention_count || 10;
      if (E.panelLogRetentionDays)
        E.panelLogRetentionDays.value = c.log_retention_days || 30;
      if (E.panelLogMaxMB) E.panelLogMaxMB.value = c.log_max_mb || 2048;
      if (E.panelMetricsRefresh)
        E.panelMetricsRefresh.value = c.metrics_refresh_seconds || 2;
      const panelFileManagerEnabled = $("panelFileManagerEnabled");
      const panelFileUploadMaxMB = $("panelFileUploadMaxMB");
      const panelLoginProtectEnabled = $("panelLoginProtectEnabled");
      const panelLoginProtectMaxAttempts = $("panelLoginProtectMaxAttempts");
      const panelLoginProtectWindow = $("panelLoginProtectWindow");
      const panelLoginProtectBlock = $("panelLoginProtectBlock");
      const panelLoginProtectMaxBuckets = $("panelLoginProtectMaxBuckets");
      const panelLoginProtectBucketIdleTTL = $("panelLoginProtectBucketIdleTTL");
      const panelLoginProtectCleanupInterval = $("panelLoginProtectCleanupInterval");
      const panelSessionMaxEntries = $("panelSessionMaxEntries");
      const panelSessionCleanupInterval = $("panelSessionCleanupInterval");
      if (panelFileManagerEnabled)
        panelFileManagerEnabled.checked = !!(
          c.file_manager && c.file_manager.enabled
        );
      if (panelFileUploadMaxMB)
        panelFileUploadMaxMB.value =
          (c.file_manager && c.file_manager.upload_max_mb) || 2048;
      if (panelLoginProtectEnabled)
        panelLoginProtectEnabled.checked = !!(
          c.login_protect && c.login_protect.enabled
        );
      if (panelLoginProtectMaxAttempts)
        panelLoginProtectMaxAttempts.value =
          (c.login_protect && c.login_protect.max_attempts) || 20;
      if (panelLoginProtectWindow)
        panelLoginProtectWindow.value =
          (c.login_protect && c.login_protect.window_seconds) || 600;
      if (panelLoginProtectBlock)
        panelLoginProtectBlock.value =
          (c.login_protect && c.login_protect.block_seconds) || 600;
      if (panelLoginProtectMaxBuckets)
        panelLoginProtectMaxBuckets.value =
          (c.login_protect && c.login_protect.max_buckets) || 10000;
      if (panelLoginProtectBucketIdleTTL)
        panelLoginProtectBucketIdleTTL.value =
          (c.login_protect && c.login_protect.bucket_idle_ttl_seconds) || 3600;
      if (panelLoginProtectCleanupInterval)
        panelLoginProtectCleanupInterval.value =
          (c.login_protect && c.login_protect.cleanup_interval_seconds) || 300;
      if (panelSessionMaxEntries)
        panelSessionMaxEntries.value = c.session_max_entries || 10000;
      if (panelSessionCleanupInterval)
        panelSessionCleanupInterval.value = c.session_cleanup_interval_seconds || 300;
      S.metricsRefreshSeconds = Number(c.metrics_refresh_seconds || 2);
      S.disableHTTPSWarning = !!c.disable_https_warning;
      dashPolling();
      updateHTTPSNotice();
      E.panelRawConfig.value = d.raw || E.panelRawConfig.value;
      E.panelRawMeta.textContent = tx("panel.settings.raw_meta").replace(
        "{path}",
        d.config_path || "config.yaml",
      );
      toast(tx("panel.settings.saved"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function normServers(s) {
    return (s || [])
      .map((v) =>
        typeof v === "string"
          ? { name: v, url: v, selected: false, base_name: v }
          : {
              name: v.name || v.url || tx("create.lagrange.sign.server"),
              url: v.url || v.name || "",
              selected: !!v.selected,
              base_name: v.base_name || v.name || v.url || "",
            },
      )
      .filter((v) => v.url);
  }
  function signVersionScore(v) {
    const nums = String(v || "")
      .match(/\d+/g);
    if (!nums || !nums.length) return 0;
    return nums.reduce((acc, n) => acc * 1000 + Number(n || 0), 0);
  }
  function sortedSigns() {
    return [...(S.sign || [])].sort((a, b) => {
      const av = String(a && a.version ? a.version : "");
      const bv = String(b && b.version ? b.version : "");
      const sa = signVersionScore(av);
      const sb = signVersionScore(bv);
      if (sa !== sb) return sb - sa;
      return bv.localeCompare(av);
    });
  }
  function signOpts(sel) {
    let ver = "",
      srv = "";
    const items = sortedSigns();
    items.forEach((it, i) => {
      const ss = normServers(it.servers),
        key = it.version || String(i),
        hit = ss.find((x) => x.url === sel);
      if (hit) {
        ver = key;
        srv = hit.url;
      }
      if (!ver && it.selected) {
        ver = key;
        const f = ss.find((x) => x.selected) || ss[0];
        srv = f ? f.url : "";
      }
      if (!ver) {
        const h = ss.find((x) => x.selected);
        if (h) {
          ver = key;
          srv = h.url;
        }
      }
    });
    if (!ver && items[0]) {
      ver = items[0].version || "default";
      const f = normServers(items[0].servers)[0];
      srv = f ? f.url : "";
    }
    return { ver, srv };
  }
  function srvOpts(ver, sel) {
    const hit = sortedSigns().find(
      (it, i) => (it.version || String(i)) === ver,
    );
    return normServers(hit ? hit.servers : [])
      .map(
        (v) =>
          `<option value="${esc(v.url)}" ${v.url === sel ? "selected" : ""}>${esc(v.name)}</option>`,
      )
      .join("");
  }
  function lagrangeVersionOptions() {
    const items = Array.isArray(S.lagrangeVersions) && S.lagrangeVersions.length
      ? S.lagrangeVersions
      : [
          { key: "latest", version: "latest", is_latest: true },
          { key: "feb_13_ddda0a6", version: "feb_13_ddda0a6", is_latest: false },
        ];
    return items
      .map((it) => {
        const key = String(it.key || it.version || "latest");
        const ver = String(it.version || key);
        const selected = key === S.lagrangeDefaultVersion ? "selected" : "";
        return `<option value="${esc(key)}" ${selected}>${esc(ver)}</option>`;
      })
      .join("");
  }

  function createCard(title, bodyHTML, note = "") {
    return `<section class="stack-block card-block config-card"><h3>${title}</h3>${bodyHTML}${note ? `<p class="muted create-card-note">${note}</p>` : ""}</section>`;
  }
  function createSealdiceUI() {
    const body = `<div class="form-grid config-grid"><div><label>${tx("create.source.label")}</label><select id="sealSource"><option value="auto">${tx("create.source.auto")}</option><option value="url">${tx("create.source.url")}</option><option value="upload">${tx("create.source.upload")}</option></select></div><div><label>${tx("create.webui_port.label")}</label><input id="sealPort" type="number" min="1" max="65535" value="3211"><div id="sealPortHint" class="port-hint"></div></div><div id="sealUrlWrap" class="hidden config-span-full"><label>${tx("create.url.label")}</label><input id="sealURL" type="text" placeholder="https://..."></div><div id="sealUploadWrap" class="hidden config-span-full"><label>${tx("create.sealdice.package")}</label><input id="sealPackage" type="file" accept=".zip,.tar,.gz,.tgz"></div></div>`;
    return createCard(
      TXT_SAFE("create.group.deploy", "部署 / 运行配置"),
      body,
      tx("create.sealdice.notice"),
    );
  }
  function createLagrangeUI() {
    const x = signOpts("");
    const signItems = sortedSigns();
    const verOpts = signItems
      .map((it, i) => {
        const v = it.version || String(i);
        return `<option value="${esc(v)}" ${v === x.ver ? "selected" : ""}>${esc(v)}</option>`;
      })
      .join("");
    const deployCard = createCard(
      TXT_SAFE("create.group.deploy", "部署 / 运行配置"),
      `<div class="form-grid config-grid"><div><label>${tx("create.source.label")}</label><select id="lagSource"><option value="auto" selected>${tx("create.source.auto")}</option><option value="url">${tx("create.source.url")}</option><option value="upload">${tx("create.source.upload")}</option></select></div><div><label>${tx("create.lagrange.version.label")}</label><select id="lagVersion"></select></div><div id="lagUploadWrap" class="config-span-full"><label>${tx("create.lagrange.package")}</label><input id="lagPackage" type="file" accept=".zip"></div><div id="lagUrlWrap" class="config-span-full hidden"><label>${tx("create.url.label")}</label><input id="lagURL" type="text" placeholder="https://..."></div><div class="config-span-full"><div class="impl-grid"><div class="impl-card"><label class="checkbox-row checkbox-row-switch"><input id="lagEnableForward" class="force-on-switch" type="checkbox" checked disabled><span>${tx("create.lagrange.impl.forward.required")}</span></label><div id="lagForwardPortWrap" class="impl-port-wrap"><label>${tx("create.port.label")}</label><input id="lagPort" type="number" min="1" max="65535" value="3212"><div id="lagPortHint" class="port-hint"></div></div></div><div class="impl-card"><label class="checkbox-row checkbox-row-switch"><input id="lagEnableReverse" type="checkbox"><span>${tx("create.lagrange.impl.reverse")}</span></label><div id="lagReversePortWrap" class="impl-port-wrap hidden"><label>${tx("create.port.label")}</label><input id="lagReversePort" type="number" min="1" max="65535" value="3213"><div id="lagReversePortHint" class="port-hint"></div></div></div><div class="impl-card"><label class="checkbox-row checkbox-row-switch"><input id="lagEnableHTTP" type="checkbox"><span>${tx("create.lagrange.impl.http")}</span></label><div id="lagHTTPPortWrap" class="impl-port-wrap hidden"><label>${tx("create.port.label")}</label><input id="lagHTTPPort" type="number" min="1" max="65535" value="3214"><div id="lagHTTPPortHint" class="port-hint"></div></div></div></div></div></div>`,
      tx("create.lagrange.notice"),
    );
    const signCard = createCard(
      TXT_SAFE("create.group.sign", "签名相关"),
      `<div class="form-grid config-grid"><div><label>${tx("create.lagrange.sign.version.label")}</label><select id="lagSignVersion">${verOpts}<option value="custom">${tx("create.sign.custom")}</option></select></div><div id="lagSignServerSelectWrap"><label>${tx("create.lagrange.sign.label")}</label><select id="lagSignServer">${srvOpts(x.ver, x.srv)}</select></div><div id="lagSignServerCustomWrap" class="hidden"><label>${tx("create.lagrange.sign.custom_url")}</label><input id="lagSignCustom" type="text" placeholder="${tx("create.lagrange.sign.custom_placeholder")}"></div><div class="config-span-full inline-tools"><button id="lagSignProbeBtn" class="btn btn-soft" type="button">${tx("create.lagrange.sign.test_all")}</button><button id="lagSignProbeCustomBtn" class="btn btn-soft hidden" type="button">${tx("create.lagrange.sign.test_one")}</button><div id="lagSignProbeResult" class="muted"></div></div></div>`,
    );
    return deployCard + signCard;
  }
  function createLLBotUI() {
    const body = `<div class="form-grid config-grid"><div><label>${tx("create.source.label")}</label><select id="llSource"><option value="auto">${tx("create.source.auto")}</option><option value="url">${tx("create.source.url")}</option><option value="upload">${tx("create.source.upload")}</option></select></div><div><label>${tx("create.llbot.version.label")}</label><select id="llVersion"><option value="latest">latest</option></select></div><div><label>${tx("create.webui_port.label")}</label><input id="llPort" type="number" min="1" max="65535" value="3215"><div id="llPortHint" class="port-hint"></div></div><div id="llUrlWrap" class="hidden config-span-full"><label>${tx("create.url.label")}</label><input id="llURL" type="text" placeholder="https://..."></div><div id="llUploadWrap" class="hidden config-span-full"><label>${tx("create.llbot.package")}</label><input id="llPackage" type="file" accept=".zip"></div><div class="config-span-full"><button id="llQQManageBtn" class="btn btn-soft" type="button">${tx("create.llbot.qq_manage")}</button></div></div>`;
    return createCard(
      TXT_SAFE("create.group.deploy", "部署 / 运行配置"),
      body,
      LLN(),
    );
  }
  function typeUI() {
    const t = E.createType.value;
    if (t === "Sealdice") return createSealdiceUI();
    if (t === "Lagrange") return createLagrangeUI();
    return createLLBotUI();
  }
  function sourceText(v) {
    if (v === "upload") return tx("create.source.upload");
    if (v === "url") return tx("create.source.url");
    return tx("create.source.auto");
  }
  function sum() {
    const t = E.createType.value,
      L = [
        tx("create.type.summary.type").replace("{value}", typ(t)),
        tx("create.type.summary.registry").replace(
          "{value}",
          E.createRegistry.value.trim() || "-",
        ),
        tx("create.type.summary.display").replace(
          "{value}",
          E.createDisplay.value.trim() || E.createRegistry.value.trim() || "-",
        ),
        tx("create.type.summary.autostart").replace(
          "{value}",
          E.createAutoStart.checked ? tx("common.yes") : tx("common.no"),
        ),
        E.createRestartEnabled.checked
          ? tx("create.type.summary.restart_on")
              .replace("{delay}", E.createRestartDelay.value || 0)
              .replace("{max}", E.createRestartMax.value || 0)
          : tx("create.type.summary.restart_off"),
      ];
    if (t === "Sealdice") {
      L.push(
        tx("create.type.summary.source").replace(
          "{value}",
          sourceText($("sealSource").value),
        ),
      );
      L.push(
        tx("create.type.summary.port").replace(
          "{value}",
          $("sealPort").value || "-",
        ),
      );
    } else if (t === "Lagrange") {
      L.push(
        tx("create.type.summary.source").replace(
          "{value}",
          sourceText($("lagSource").value),
        ),
      );
      L.push(
        tx("create.type.summary.version").replace(
          "{value}",
          $("lagVersion").value,
        ),
      );
      L.push(
        tx("create.type.summary.forward").replace(
          "{value}",
          $("lagPort").value || "-",
        ),
      );
      L.push(
        tx("create.type.summary.reverse").replace(
          "{value}",
          $("lagEnableReverse").checked
            ? $("lagReversePort").value
            : tx("common.no"),
        ),
      );
      L.push(
        tx("create.type.summary.http").replace(
          "{value}",
          $("lagEnableHTTP").checked ? $("lagHTTPPort").value : tx("common.no"),
        ),
      );
    } else {
      L.push(
        tx("create.type.summary.source").replace(
          "{value}",
          sourceText($("llSource").value),
        ),
      );
      L.push(
        tx("create.type.summary.version").replace(
          "{value}",
          $("llVersion").value,
        ),
      );
      L.push(
        tx("create.type.summary.webui").replace(
          "{value}",
          $("llPort").value || "-",
        ),
      );
    }
    E.createSummary.textContent = L.join("\n");
  }
  function bindType() {
    const t = E.createType.value;
    if (t === "Sealdice") {
      S.createGetSignURL = null;
      ["sealSource", "sealPort", "sealURL", "sealPackage"].forEach((id) => {
        const x = $(id);
        if (!x) return;
        x.onchange = sum;
        x.oninput = sum;
      });
      $("sealSource").onchange = () => {
        const source = $("sealSource").value;
        $("sealUploadWrap").classList.toggle("hidden", source !== "upload");
        $("sealUrlWrap").classList.toggle("hidden", source !== "url");
        sum();
      };
      bindPortFieldRealtime("sealPort", "sealPortHint", "");
    } else if (t === "Lagrange") {
      [
        "lagSource",
        "lagVersion",
        "lagPackage",
        "lagURL",
        "lagPort",
        "lagEnableReverse",
        "lagReversePort",
        "lagEnableHTTP",
        "lagHTTPPort",
        "lagSignCustom",
      ].forEach((id) => {
        const el = $(id);
        if (!el) return;
        el.onchange = sum;
        el.oninput = sum;
      });

      const isCustomSign = () => $("lagSignVersion").value === "custom";
      const getCreateSignURL = () =>
        isCustomSign()
          ? String($("lagSignCustom").value || "").trim()
          : String($("lagSignServer").value || "").trim();

      const updateSignMode = () => {
        const custom = isCustomSign();
        $("lagSignServerSelectWrap").classList.toggle("hidden", custom);
        $("lagSignServerCustomWrap").classList.toggle("hidden", !custom);
        $("lagSignProbeBtn").classList.toggle("hidden", custom);
        $("lagSignProbeCustomBtn").classList.toggle("hidden", !custom);
      };
      const refreshLagrangeSource = () => {
        const source = $("lagSource").value;
        const upload = source === "upload";
        const byURL = source === "url";
        $("lagUploadWrap").classList.toggle("hidden", !upload);
        $("lagUrlWrap").classList.toggle("hidden", !byURL);
      };

      const refreshSignServers = async () => {
        updateSignMode();
        if (isCustomSign()) {
          $("lagSignProbeResult").textContent = "";
          sum();
          return;
        }
        $("lagSignServer").innerHTML = srvOpts($("lagSignVersion").value, "");
        await probeCurrentSign("lagSignServer", "lagSignProbeResult");
        sum();
      };

      $("lagSignVersion").onchange = refreshSignServers;
      $("lagSource").onchange = () => {
        refreshLagrangeSource();
        sum();
      };
      $("lagSignServer").onchange = async () => {
        if (!isCustomSign()) {
          await probeCurrentSign("lagSignServer", "lagSignProbeResult");
        }
        sum();
      };

      $("lagSignProbeBtn").onclick = probe;
      $("lagSignProbeCustomBtn").onclick = async () => {
        const url = getCreateSignURL();
        if (!url) {
          $("lagSignProbeResult").textContent = tx("create.lagrange.no_servers");
          return;
        }
        $("lagSignProbeResult").textContent = tx("create.lagrange.ping_checking");
        const r = await probeSignLatency(url, 5);
        $("lagSignProbeResult").textContent = r.ok
          ? tx("create.lagrange.ping_ok").replace("{avg}", r.avg_ms)
          : tx("create.lagrange.ping_fail");
      };

      const lagVersion = $("lagVersion");
      if (lagVersion) lagVersion.innerHTML = lagrangeVersionOptions();
      refreshSignServers();
      refreshLagrangeSource();
      setTimeout(() => {
        if (!isCustomSign() && $("lagSignProbeBtn")) probe();
      }, 60);

      bindPortFieldRealtime("lagPort", "lagPortHint", "");
      bindPortFieldRealtime("lagReversePort", "lagReversePortHint", "");
      bindPortFieldRealtime("lagHTTPPort", "lagHTTPPortHint", "");
      $("lagEnableReverse").addEventListener("change", () => {
        $("lagReversePortWrap").classList.toggle(
          "hidden",
          !$("lagEnableReverse").checked,
        );
        if (!$("lagEnableReverse").checked)
          setPortHint($("lagReversePort"), $("lagReversePortHint"), "", false);
        sum();
      });
      $("lagEnableHTTP").addEventListener("change", () => {
        $("lagHTTPPortWrap").classList.toggle("hidden", !$("lagEnableHTTP").checked);
        if (!$("lagEnableHTTP").checked)
          setPortHint($("lagHTTPPort"), $("lagHTTPPortHint"), "", false);
        sum();
      });

      S.createGetSignURL = getCreateSignURL;
    } else {
      const src = $("llSource");

      const llUploadWrap = $("llUploadWrap");
      S.createGetSignURL = null;
      $("llSource").onchange = () => {
        const source = src.value;
        const upload = source === "upload";
        const byURL = source === "url";
        llUploadWrap.classList.toggle("hidden", !upload);
        $("llUrlWrap").classList.toggle("hidden", !byURL);

        sum();
      };
      $("llPort").oninput = sum;
      $("llVersion").onchange = sum;
      $("llSource").oninput = sum;
      if ($("llURL")) $("llURL").oninput = sum;

      if ($("llPackage")) $("llPackage").onchange = sum;
      $("llQQManageBtn").onclick = () => qqOpen(true);
      bindPortFieldRealtime("llPort", "llPortHint", "");
      $("llSource").onchange();
    }
    sum();
  }
  function createRender() {
    E.createTypeSpecific.innerHTML = typeUI();
    bindType();
  }
  function openCreate() {
    S.createGetSignURL = null;
    E.createRegistry.value = "";
    E.createDisplay.value = "";
    E.createAutoStart.checked = false;
    E.createRestartEnabled.checked = false;
    E.createRestartDelay.value = 3;
    E.createRestartMax.value = 3;
    E.createType.value = "Sealdice";
    createRender();
    show(E.createModal);
  }
  async function probe() {
    const v = $("lagSignVersion").value,
      hit = (S.sign || []).find((it, i) => (it.version || String(i)) === v),
      ss = normServers(hit ? hit.servers : []);
    if (!ss.length) {
      $("lagSignProbeResult").textContent = tx("create.lagrange.no_servers");
      return;
    }
    $("lagSignProbeResult").textContent = tx("create.lagrange.probing");
    let ok = 0;
    await Promise.all(
      ss.map(async (s) => {
        const baseName = s.base_name || s.name || s.url;
        const r = await probeSignLatency(s.url, 5);
        s.base_name = baseName;
        s.name = r.ok
          ? `${baseName} (${r.avg_ms}ms)`
          : `${baseName} (${tx("create.lagrange.failed_short")})`;
        if (r.ok) ok++;
      }),
    );
    if (hit) hit.servers = ss;
    $("lagSignServer").innerHTML = srvOpts(v, $("lagSignServer").value);
    $("lagSignProbeResult").textContent = tx("create.lagrange.probe_done")
      .replace("{ok}", ok)
      .replace("{total}", ss.length);
  }

  async function probeSignLatency(url, times = 5) {
    const n = Math.max(1, Number(times) || 1);
    const tasks = Array.from({ length: n }, async () => {
      try {
        const r = await j("deploy/lagrange/sign-probe", {
          method: "POST",
          body: JSON.stringify({ url }),
        });
        return { ok: !!r.ok, ms: Number(r.avg_ms || 0) };
      } catch (_) {
        return { ok: false, ms: 0 };
      }
    });
    const results = await Promise.all(tasks);
    const oks = results.filter((x) => x.ok);
    if (!oks.length) return { ok: false, avg_ms: 0 };
    const avg = Math.round(oks.reduce((a, b) => a + b.ms, 0) / oks.length);
    return { ok: true, avg_ms: avg };
  }

  async function probeCurrentSign(selectID, textID) {
    const sel = $(selectID),
      out = $(textID);
    if (!sel || !out || !sel.value) {
      if (out) out.textContent = "";
      return;
    }
    out.textContent = tx("create.lagrange.ping_checking");
    const r = await probeSignLatency(sel.value, 5);
    out.textContent = r.ok
      ? tx("create.lagrange.ping_ok").replace("{avg}", r.avg_ms)
      : tx("create.lagrange.ping_fail");
  }
  async function submitCreate() {
    try {
      const t = E.createType.value,
        id = E.createRegistry.value.trim(),
        name = E.createDisplay.value.trim() || id,
        auto = E.createAutoStart.checked,
        rs = {
          enabled: E.createRestartEnabled.checked,
          delay_seconds: Number(E.createRestartDelay.value || 0),
          max_crash_count: Number(E.createRestartMax.value || 0),
        };
      if (!/^[A-Za-z0-9_-]+$/.test(id))
        throw new Error(tx("error.registry_invalid"));
      let r;
      if (t === "Sealdice") {
        const chk = await validatePortField("sealPort", "sealPortHint", "");
        if (!chk.ok) throw new Error(chk.message || tx("error.invalid_port"));
        const port = chk.port;
        await showWarning(tx("notice.cloud_port"));
        const sealSource = $("sealSource").value;
        if (sealSource === "upload") {
          const f = $("sealPackage").files[0];
          if (!f) throw new Error(tx("toast.choose_archive_first"));
          const fd = new FormData();
          [
            ["registry_name", id],
            ["display_name", name],
            ["port", String(port)],
            ["auto_start", String(auto)],
            ["restart_enabled", String(rs.enabled)],
            ["restart_delay_seconds", String(rs.delay_seconds)],
            ["restart_max_crash_count", String(rs.max_crash_count)],
          ].forEach(([k, v]) => fd.append(k, v));
          fd.append("package", f);
          r = await j("deploy/sealdice/upload", { method: "POST", body: fd });
        } else {
          const sealURL = String($("sealURL").value || "").trim();
          if (sealSource === "url" && !sealURL) throw new Error(tx("create.url.required"));
          r = await j("deploy/sealdice/auto", {
            method: "POST",
            body: JSON.stringify({
              source: sealSource,
              url: sealSource === "url" ? sealURL : "",
              registry_name: id,
              display_name: name,
              port,
              auto_start: auto,
              restart: rs,
            }),
          });
        }
      } else if (t === "Lagrange") {
        const pForward = await validatePortField("lagPort", "lagPortHint", "");
        if (!pForward.ok)
          throw new Error(pForward.message || tx("error.invalid_port"));
        let reversePort = 0,
          httpPort = 0;
        if ($("lagEnableReverse").checked) {
          const pReverse = await validatePortField(
            "lagReversePort",
            "lagReversePortHint",
            "",
          );
          if (!pReverse.ok)
            throw new Error(pReverse.message || tx("error.invalid_port"));
          reversePort = pReverse.port;
        }
        if ($("lagEnableHTTP").checked) {
          const pHTTP = await validatePortField(
            "lagHTTPPort",
            "lagHTTPPortHint",
            "",
          );
          if (!pHTTP.ok)
            throw new Error(pHTTP.message || tx("error.invalid_port"));
          httpPort = pHTTP.port;
        }
        const signURL = String((S.createGetSignURL && S.createGetSignURL()) || "").trim();
        if (!signURL) throw new Error(tx("create.lagrange.no_servers"));
        await showWarning(tx("notice.cloud_port"));
        const lagSource = $("lagSource").value;
        if (lagSource === "upload") {
          const f = $("lagPackage").files[0];
          if (!f) throw new Error(tx("toast.choose_archive_first"));
          const fd = new FormData();
          [
            ["registry_name", id],
            ["display_name", name],
            ["auto_start", String(auto)],
            ["version", $("lagVersion").value],
            ["forward_ws_port", String(pForward.port)],
            ["enable_reverse_ws", String($("lagEnableReverse").checked)],
            ["reverse_ws_port", String(reversePort)],
            ["enable_http", String($("lagEnableHTTP").checked)],
            ["http_port", String(httpPort)],
            ["sign_server_url", signURL],
            ["restart_enabled", String(rs.enabled)],
            ["restart_delay_seconds", String(rs.delay_seconds)],
            ["restart_max_crash_count", String(rs.max_crash_count)],
          ].forEach(([k, v]) => fd.append(k, v));
          fd.append("package", f);
          r = await j("deploy/lagrange/upload", { method: "POST", body: fd });
        } else {
          const lagURL = String($("lagURL").value || "").trim();
          if (lagSource === "url" && !lagURL) throw new Error(tx("create.url.required"));
          r = await j("deploy/lagrange/auto", {
            method: "POST",
            body: JSON.stringify({
              source: lagSource,
              registry_name: id,
              display_name: name,
              auto_start: auto,
              version: $("lagVersion").value,
              download_url: lagSource === "url" ? lagURL : "",
              port: pForward.port,
              enable_forward_ws: true,
              forward_ws_port: pForward.port,
              sign_server_url: signURL,
              enable_reverse_ws: $("lagEnableReverse").checked,
              reverse_ws_port: reversePort,
              enable_http: $("lagEnableHTTP").checked,
              http_port: httpPort,
              restart: rs,
            }),
          });
        }
      } else {
        const qs = await j("llbot/qq/status");
        if (!qs.installed) {
          qqOpen(true);
          throw new Error(tx("qq.modal.need_before_deploy"));
        }
        const chk = await validatePortField("llPort", "llPortHint", "");
        if (!chk.ok) throw new Error(chk.message || tx("error.invalid_port"));
        const port = chk.port;
        await showWarning(tx("notice.cloud_port"));
        const llSource = $("llSource").value;
        if (llSource === "upload") {
          const f = $("llPackage").files[0];
          if (!f) throw new Error(tx("toast.choose_archive_first"));
          const fd = new FormData();
          [
            ["registry_name", id],
            ["display_name", name],
            ["port", String(port)],
            ["auto_start", String(auto)],
            ["restart_enabled", String(rs.enabled)],
            ["restart_delay_seconds", String(rs.delay_seconds)],
            ["restart_max_crash_count", String(rs.max_crash_count)],
          ].forEach(([k, v]) => fd.append(k, v));
          fd.append("package", f);
          r = await j("deploy/llbot/upload", { method: "POST", body: fd });
        } else {
          const llURL = String($("llURL").value || "").trim();
          if (llSource === "url" && !llURL) throw new Error(tx("create.url.required"));
          r = await j("deploy/llbot/auto", {
            method: "POST",
            body: JSON.stringify({
              source: llSource,
              url: llSource === "url" ? llURL : "",
              registry_name: id,
              display_name: name,
              port,
              auto_start: auto,
              version: $("llVersion").value,
              restart: rs,
            }),
          });
        }
      }
      hide(E.createModal);
      if (r.deploy_log) depOpen(r.deploy_log, true);
      await loadSvcs(true);
      if (r.service && r.service.id) {
        view("services");
        openSvc(r.service.id);
      }
      toast(tx("toast.deploy_done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function depOpen(name, auto) {
    S.deploy.name = name;
    E.deployLogBox.textContent = "";
    show(E.deployLogModal);
    if (S.deploy.timer) clearInterval(S.deploy.timer);
    const tick = async () => {
      try {
        const d = await j(`deploy/logs/${encodeURIComponent(name)}?lines=500`);
        setPre(E.deployLogBox, d.content || "");
        if (
          auto &&
          /deploy: success|deploy: rebuild complete|qq: install success/i.test(
            d.content || "",
          )
        ) {
          clearInterval(S.deploy.timer);
          S.deploy.timer = 0;
          setTimeout(() => hide(E.deployLogModal), 900);
        }
      } catch (e) {
        setPre(
          E.deployLogBox,
          tx("deploy.logs.load_failed") + ": " + e.message,
        );
      }
    };
    S.deploy.timer = setInterval(tick, 1200);
    tick();
  }
  function depClose() {
    if (S.deploy.timer) clearInterval(S.deploy.timer);
    S.deploy.timer = 0;
    hide(E.deployLogModal);
  }
  async function qqOpen(fromCreate = false) {
    show(E.qqModal);
    E.qqLogBox.textContent = "";
    try {
      const d = await j("llbot/qq/status");
      E.qqStatusText.textContent = d.installed
        ? tx("qq.modal.status.installed").replace("{path}", d.path)
        : tx("qq.modal.status.missing").replace("{path}", d.path);
      E.qqUrl.value = d.default || "";
      if (fromCreate && !d.installed)
        toast(tx("qq.modal.need_before_deploy"), "info");
    } catch (e) {
      E.qqStatusText.textContent =
        tx("qq.status.load_failed") + ": " + e.message;
    }
  }
  function qqPoll(name) {
    if (S.qq.timer) clearInterval(S.qq.timer);
    S.qq.name = name;
    const tick = async () => {
      try {
        const d = await j(`deploy/logs/${encodeURIComponent(name)}?lines=500`);
        setPre(E.qqLogBox, d.content || "");
        if (/qq: install success/i.test(d.content || "")) {
          clearInterval(S.qq.timer);
          S.qq.timer = 0;
          toast(tx("qq.modal.install_done"), "ok");
          qqOpen(false);
        }
      } catch (e) {
        setPre(E.qqLogBox, tx("deploy.logs.load_failed") + ": " + e.message);
      }
    };
    S.qq.timer = setInterval(tick, 1200);
    tick();
  }
  async function qqInstall() {
    try {
      const d = await j("llbot/qq/install", {
        method: "POST",
        body: JSON.stringify({ url: E.qqUrl.value.trim() }),
      });
      if (d.deploy_log) qqPoll(d.deploy_log);
    } catch (e) {
      toast(e.message, "error");
    }
  }
  function qr() {
    if (!S.svc) return;
    E.qrImage.src = `${ep(`services/${encodeURIComponent(S.svc.id)}/qrcode`)}?_=${Date.now()}`;
    show(E.qrModal);
  }
  async function delOpen() {
    if (!S.svc) return;
    const message =
      `${tx("confirm.delete_service")}\n` +
      `${tx("confirm.service_registry_label")}: ${S.svc.id}`;
    const ok = await openConfirmDialog({
      title: tx("detail.delete"),
      message,
      requireInput: true,
      expectedInput: S.svc.id,
      inputLabel: `${tx("confirm.input_label")} (${S.svc.id})`,
      submitText: tx("confirm.submit"),
      danger: true,
      showCancel: true,
      warning: false,
    });
    if (!ok) return;
    try {
      await j("services/" + encodeURIComponent(S.svc.id) + "/delete", {
        method: "POST",
      });
      closeSvc();
      await loadSvcs(true);
      toast(tx("msg.delete_done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function rebuild() {
    if (!S.svc) return;
    try {
      const info = await j(
        `services/${encodeURIComponent(S.svc.id)}/rebuild-info`,
      );
      let mode = "";
      const source = String((info && info.source) || "auto").toLowerCase();
      if (source === "upload") {
        const useUpload = await openConfirmDialog({
          title: tx("detail.rebuild"),
          message: tx(
            "confirm.rebuild_upload_source",
            "该服务来自你上传的安装包。确定：按上传包重建；关闭：改为自动下载重建。",
          ),
          submitText: tx("confirm.rebuild_use_upload", "使用上传来源"),
          danger: true,
          showCancel: true,
          warning: false,
        });
        if (useUpload) {
          mode = "upload";
        } else {
          const useAuto = await openConfirmDialog({
            title: tx("detail.rebuild"),
            message: tx(
              "confirm.rebuild_use_auto_desc",
              "将改用直接下载重建，是否继续？",
            ),
            submitText: tx("confirm.rebuild_use_auto", "使用直接下载"),
            danger: false,
            showCancel: true,
            warning: true,
          });
          if (!useAuto) return;
          mode = "auto";
        }
      } else if (!(await askConfirm(tx("confirm.rebuild")))) {
        return;
      }
      const d = await j(`services/${encodeURIComponent(S.svc.id)}/rebuild`, {
        method: "POST",
        body: JSON.stringify({ mode }),
      });
      if (d.deploy_log) depOpen(d.deploy_log, true);
      await loadSvcs(true);
      toast(tx("detail.rebuild") + " " + tx("toast.done"), "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  }
  async function lagrangeVersionsLoad() {
    try {
      const d = await j("deploy/lagrange/versions");
      S.lagrangeVersions = Array.isArray(d.items) ? d.items : [];
      S.lagrangeDefaultVersion = String(d.default_stable || "latest");
    } catch (_) {
      S.lagrangeVersions = [];
      S.lagrangeDefaultVersion = "latest";
    }
  }

  async function signLoad() {
    try {
      const d = await j("deploy/lagrange/signinfo");
      S.sign = Array.isArray(d.items) ? d.items : [];
    } catch (_) {
      S.sign = [];
    }
  }
  function bind() {
    E.detailForceBtn.textContent = tx("detail.force_stop");
    E.detailEntryBtn.textContent = tx("detail.entry.open");
    E.detailConfigBtn.textContent = tx("detail.config_center");
    E.initSubmit.onclick = async () => {
      try {
        await j("auth/init", {
          method: "POST",
          body: JSON.stringify({ password: E.initPassword.value }),
        });
        await j("auth/login", {
          method: "POST",
          body: JSON.stringify({ password: E.initPassword.value }),
        });
        await enter();
      } catch (e) {
        toast(e.message, "error");
      }
    };
    E.loginSubmit.onclick = async () => {
      try {
        await j("auth/login", {
          method: "POST",
          body: JSON.stringify({ password: E.loginPassword.value }),
        });
        await enter();
      } catch (e) {
        toast(e.message, "error");
      }
    };
    E.initPassword.onkeydown = (e) => {
      if (e.key === "Enter") E.initSubmit.click();
    };
    E.loginPassword.onkeydown = (e) => {
      if (e.key === "Enter") E.loginSubmit.click();
    };
    E.navDashboard.onclick = () => view("dashboard");
    E.navServices.onclick = () => {
      if (S.svc) closeSvc();
      view("services");
      const t = appPath("/services");
      if (location.pathname !== t) {
        history.pushState(null, "", t);
        window.dispatchEvent(new PopStateEvent("popstate"));
      }
    };
    if (E.navServicesToggle)
      E.navServicesToggle.onclick = () => {
        S.tree = !S.tree;
        E.serviceTreeWrap.classList.toggle("hidden", !S.tree);
        if (E.navServices.classList.contains("active"))
          E.navServicesToggle.classList.add("active");
      };
    E.navSettings.onclick = () => view("settings");
    E.refreshBtn.onclick = () => loadSvcs();
    E.logoutBtn.onclick = async () => {
      try {
        await j("auth/logout", { method: "POST" });
      } catch (_) {}
      E.appShell.classList.add("ui-leave");
      setTimeout(() => location.reload(), 140);
    };
    E.globalCreateBtn.onclick = openCreate;
    E.dashboardToServices.onclick = () => {
      if (S.svc) closeSvc();
      view("services");
      const t = appPath("/services");
      if (location.pathname !== t) {
        history.pushState(null, "", t);
        window.dispatchEvent(new PopStateEvent("popstate"));
      }
    };
    E.servicesCreateBtn.onclick = openCreate;
    [
      E.serviceSearch,
      E.serviceTypeFilter,
      E.serviceStatusFilter,
      E.serviceSort,
    ].forEach((x) => {
      x.oninput = cards;
      x.onchange = cards;
    });
    E.detailBackBtn.onclick = closeSvc;
    E.detailStartBtn.onclick = () => svcAct("start");
    E.detailStopBtn.onclick = () => svcAct("stop");
    E.detailRestartBtn.onclick = () => svcAct("restart");
    E.detailForceBtn.onclick = async () => {
      if (await askConfirm(tx("confirm.force_stop"))) svcAct("force-stop");
    };
    E.detailEntryBtn.onclick = () => {
      if (!S.svc) return;
      if (S.svc.type === "Lagrange") qr();
      else
        window.open(
          (S.svc.open_path_url || "").trim() ||
            `${location.protocol}//${location.hostname}:${S.svc.port}/`,
          "_blank",
          "noopener",
        );
    };
    E.detailConfigBtn.onclick = setOpen;
    E.detailMoreBtn.onclick = () => E.detailMoreMenu.classList.toggle("hidden");
    document.addEventListener("click", (e) => {
      if (!E.detailMoreMenu.contains(e.target) && e.target !== E.detailMoreBtn)
        hide(E.detailMoreMenu);
    });
    $$(".tab-btn").forEach((b) => (b.onclick = () => tab(b.dataset.tab)));
    E.logSearch.oninput = logLoad;
    E.logPauseBtn.onclick = () => {
      S.logs.paused = !S.logs.paused;
      E.logPauseBtn.textContent = S.logs.paused
        ? tx("detail.logs.resume")
        : tx("detail.logs.pause");
      if (!S.logs.paused) logLoad();
    };
    E.logDownloadBtn.onclick = () => {
      const b = new Blob([S.logs.all || ""], {
          type: "text/plain;charset=utf-8",
        }),
        a = document.createElement("a");
      a.href = URL.createObjectURL(b);
      a.download = `${S.svc ? S.svc.id : "service"}-log.txt`;
      a.click();
      URL.revokeObjectURL(a.href);
    };
    E.logClearBtn.onclick = logLoad;
    E.logHistoryBtn.onclick = openLogHistory;
    if (E.logHistorySearch) E.logHistorySearch.oninput = renderLogHistoryContent;
    if (E.logHistoryClearBtn) E.logHistoryClearBtn.onclick = () => {
      if (!S.logHistory.name) return toast(tx("common.no_data"), "error");
      deleteHistoryItem(S.logHistory.name);
    };
    if (E.logHistoryClearAllBtn) E.logHistoryClearAllBtn.onclick = clearHistoryLogs;
    if (E.panelLogsClearBtn)
      E.panelLogsClearBtn.onclick = async () => {
        if (!(await askConfirm(tx("confirm.clear_panel_logs")))) return;
        try {
          await j("panel/logs/clear", { method: "POST" });
          toast(tx("msg.delete_done"), "ok");
        } catch (e) {
          toast(e.message, "error");
        }
      };
    E.fileSearch.oninput = fRender;
    E.fileSort.onchange = fRender;
    E.filesStopBtn.onclick = () => {};
    E.fileUpBtn.onclick = () => files(S.file.parent || "");
    E.fileRefreshBtn.onclick = () => files(S.file.path);
    E.fileUploadBtn.onclick = () => E.fileUploadInput.click();
    E.fileUploadInput.onchange = async () => {
      if (!E.fileUploadInput.files.length) return;
      const fd = new FormData();
      Array.from(E.fileUploadInput.files).forEach((f) => fd.append("files", f));
      await fPost("upload", null, {
        form: fd,
        q: `path=${encodeURIComponent(S.file.path)}`,
      });
      E.fileUploadInput.value = "";
    };
    E.fileNewDirBtn.onclick = async () => {
      const n = prompt(tx("prompt.mkdir_title"));
      if (n) await fPost("mkdir", { parent: S.file.path, name: n });
    };
    E.fileNewFileBtn.onclick = async () => {
      const n = prompt(tx("prompt.mkfile_title"));
      if (n) await fPost("mkfile", { parent: S.file.path, name: n });
    };
    E.fileCopyBtn.onclick = () => {
      S.file.clip = picked().map((x) => x.path);
      toast(tx("msg.copy_done").replace("{count}", S.file.clip.length), "ok");
    };
    E.filePasteBtn.onclick = async () => {
      if (!S.file.clip.length) {
        toast(tx("toast.clipboard_empty"), "info");
        return;
      }
      await fPost("copy", { sources: S.file.clip, destination: S.file.path });
    };
    E.fileRenameBtn.onclick = async () => {
      const a = picked();
      if (a.length !== 1) {
        toast(tx("error.select_first"), "info");
        return;
      }
      const n = prompt(tx("prompt.new_name"), a[0].name);
      if (n) await fPost("rename", { path: a[0].path, new_name: n });
    };
    E.fileCompressBtn.onclick = async () => {
      const a = picked();
      if (!a.length) {
        toast(tx("error.select_first"), "info");
        return;
      }
      const n = prompt(
        tx("prompt.archive_name"),
        tx("prompt.default_archive_name"),
      );
      if (n)
        await fPost("compress", {
          sources: a.map((x) => x.path),
          destination: S.file.path,
          output_name: n,
        });
    };
    E.fileExtractBtn.onclick = async () => {
      const a = picked();
      if (a.length !== 1 || a[0].is_dir || !isArc(a[0].name)) {
        toast(tx("error.select_one_archive"), "info");
        return;
      }
      await fPost("extract", { path: a[0].path, destination: S.file.path });
    };
    E.fileDeleteBtn.onclick = async () => {
      const a = picked();
      if (!a.length) {
        toast(tx("error.select_first"), "info");
        return;
      }
      if (await askConfirm(tx("confirm.delete").replace("{count}", a.length)))
        await fPost("delete", { paths: a.map((x) => x.path) });
    };
    E.fileSelectAll.onchange = () => {
      S.file.sel = E.fileSelectAll.checked
        ? new Set(S.file.entries.map((x) => x.path))
        : new Set();
      fRender();
    };
    E.openSettingsDrawerBtn.onclick = setOpen;
    E.openPortsDrawerBtn.onclick = setOpen;
    E.createCloseBtn.onclick = () => hide(E.createModal);
    E.createType.onchange = createRender;
    [
      E.createRegistry,
      E.createDisplay,
      E.createAutoStart,
      E.createRestartEnabled,
      E.createRestartDelay,
      E.createRestartMax,
    ].forEach((x) => {
      x.oninput = sum;
      x.onchange = sum;
    });
    E.createSubmitBtn.onclick = submitCreate;
    E.settingsDrawerCloseBtn.onclick = () => hide(E.settingsDrawer);
    E.detailResetOpenPathBtn.onclick = () => (E.detailOpenPathUrl.value = "");
    E.detailSettingsSaveBtn.onclick = setSave;
    E.portsDrawerCloseBtn.onclick = () => hide(E.portsDrawer);
    E.portsSaveBtn.onclick = portsSave;
    E.panelConfigSaveBtn.onclick = panelSaveForm;
    E.panelRawSaveBtn.onclick = panelSaveRaw;
    if (E.panelUpdateCheckBtn) E.panelUpdateCheckBtn.onclick = panelCheckUpdate;
    if (E.panelUpdateApplyBtn) E.panelUpdateApplyBtn.onclick = panelApplyUpdate;
    if (E.shellVersion) {
      E.shellVersion.onclick = () => {
        if (S.update.hasUpdate) {
          view("settings");
          return;
        }
        toast(TXT_SAFE("panel.update.latest", "当前已是最新版本"), "info");
      };
    }
    E.editorText.oninput = () => {
      S.editor.dirty = E.editorText.value !== S.editor.saved;
      edMeta();
    };
    E.editorText.onkeyup = edMeta;
    E.editorText.onclick = edMeta;
    E.editorText.onkeydown = (e) => {
      if (e.ctrlKey && e.key.toLowerCase() === "s") {
        e.preventDefault();
        edSave();
      }
    };
    E.editorSaveBtn.onclick = edSave;
    E.editorCloseBtn.onclick = edClose;
    E.deployLogCloseBtn.onclick = depClose;
    E.qqCloseBtn.onclick = () => hide(E.qqModal);
    E.qqInstallBtn.onclick = qqInstall;
    E.qqRefreshBtn.onclick = () => qqOpen(false);
    E.qrCloseBtn.onclick = () => hide(E.qrModal);
    E.confirmCancelBtn.onclick = () => closeConfirmDialog(false);
    E.logHistoryCloseBtn.onclick = () => hide(E.logHistoryModal);
    E.confirmSubmitBtn.onclick = async () => {
      if (!S.confirm) return;
      try {
        await S.confirm();
        S.confirm = null;
      } catch (e) {
        toast(e.message, "error");
      }
    };
    [
      E.createModal,
      E.settingsDrawer,
      E.portsDrawer,
      E.editorModal,
      E.deployLogModal,
      E.qqModal,
      E.qrModal,
      E.logHistoryModal,
    ].forEach(
      (o) =>
        (o.onclick = (e) => {
          if (e.target === o) hide(o);
        }),
    );
    E.confirmModal.onclick = (e) => {
      if (e.target === E.confirmModal) closeConfirmDialog(false);
    };
  }
  async function enter() {
    hide(E.authInit);
    hide(E.authLogin);
    show(E.appShell);
    S.tree = true;
    show(E.serviceTreeWrap);
    await lagrangeVersionsLoad();
    await signLoad();
    await loadSvcs();
    void panelCheckUpdateSilent();
    if (!S.entryHTTPWarnShown && !S.isSecure && !S.disableHTTPSWarning) {
      S.entryHTTPWarnShown = true;
      await showWarning(tx("status.https_warn"), tx("warning.title.danger_env"));
    }
    miniStart();
    view("dashboard");
    const u = new URL(window.location.href),
      id = u.searchParams.get("service");
    if (id) {
      view("services");
      openSvc(id);
    }
  }
  async function boot() {
    c();
    initSettingsCenter();
    if (window.textReady && typeof window.textReady.then === "function") {
      try {
        await window.textReady;
      } catch (_) {}
    }
    labels();
    bind();
    try {
      const b = await j("bootstrap/status");
      if (E.shellVersion && b) {
        const raw = String(b.version_raw || "").trim();
        E.shellVersion.textContent = String(raw || b.version || "-");
      }
      if (b.needs_password_setup) {
        show(E.authInit);
        E.initPassword.focus();
        return;
      }
      try {
        const me = await j("me");
        S.isSecure = !!(me && me.is_secure);
        S.disableHTTPSWarning = !!(me && me.disable_https_warning);
        await enter();
      } catch (_) {
        show(E.authLogin);
        E.loginPassword.focus();
      }
    } catch (e) {
      toast(tx("toast.init_failed").replace("{msg}", e.message), "error");
      show(E.authLogin);
    }
  }
  window.addEventListener("beforeunload", () => {
    logStop();
    if (S.deploy.timer) clearInterval(S.deploy.timer);
    if (S.qq.timer) clearInterval(S.qq.timer);
  });
  document.addEventListener("DOMContentLoaded", boot);
})();
