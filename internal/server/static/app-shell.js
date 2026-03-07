(function () {
  "use strict";

  const ROUTE_ROOTS = new Set(["dashboard", "settings", "services"]);
  const SERVICE_TABS = new Set(["overview", "logs", "files"]);
  const TEXT_IDS = {
    brandTitle: "app.title",
    brandSub: "app.subtitle",
    navDashboard: "nav.dashboard",
    navSettings: "nav.settings",
    navServices: "nav.services",
    shellTitle: "shell.title",
    shellSub: "shell.subtitle",
    refreshBtn: "action.refresh",
    logoutBtn: "action.logout",
    dashboardTitle: "dashboard.title",
    dashboardSub: "dashboard.subtitle",
    dashboardToServices: "dashboard.goto_services",
    dashRunningLabel: "dashboard.running",
    dashStoppedLabel: "dashboard.stopped",
    dashErrorLabel: "dashboard.error",
    dashTotalLabel: "dashboard.total",
    dashSealLabel: "dashboard.sealdice",
    dashLagrangeLabel: "dashboard.lagrange",
    dashLLBotLabel: "dashboard.llbot",
    servicesTitle: "service.overview.title",
    servicesSub: "service.overview.subtitle",
    servicesCreateBtn: "service.create",
    detailBackBtn: "detail.back",
    detailStatusLabel: "detail.hero.status",
    detailPortLabel: "detail.hero.port",
    detailPidLabel: "detail.hero.pid",
    detailStartBtn: "detail.start",
    detailStopBtn: "detail.stop",
    detailRestartBtn: "event.restart",
    detailForceBtn: "detail.force_stop",
    detailConfigBtn: "detail.config_center",
    detailMoreBtn: "action.more",
    overviewPathLabel: "detail.overview.path",
    overviewTypeLabel: "service.type",
    previewLogsTitle: "detail.overview.recent_logs",
    logsTitle: "detail.logs.title",
    filesTitle: "file.title",
    settingsTitle: "nav.settings",
    settingsSub: "settings.subtitle",
    settingsPlaceholder: "settings.placeholder",
    createTitle: "create.title",
    createSub: "create.subtitle",
    createTypeLabel: "create.type.label",
    createRegistryLabel: "create.registry.label",
    createDisplayLabel: "create.display.label",
    restartTitle: "restart.title",
    restartEnabledLabel: "restart.enabled",
    restartDelayLabel: "restart.delay",
    restartMaxLabel: "restart.max_crash",
    createSummaryTitle: "create.step.4.title",
    createSubmitBtn: "create.submit",
    settingsDrawerTitle: "detail.config_center",
    configCardOneTitle: "detail.settings.section.general.title",
    configCardOneDesc: "detail.settings.section.general.desc",
    configCardTwoTitle: "detail.settings.section.restart.title",
    configCardTwoDesc: "detail.settings.section.restart.desc",
    detailDisplayLabel: "detail.display_name",
    detailAutoStartLabel: "detail.auto_start",
    detailOpenPathLabel: "detail.open_path.custom",
    detailRestartEnabledLabel: "restart.enabled",
    detailRestartDelayLabel: "restart.delay",
    detailRestartMaxLabel: "restart.max_crash",
    detailResetOpenPathBtn: "detail.open_path.reset",
    detailSettingsSaveBtn: "detail.settings.save",
    editorTitle: "editor.title",
    editorSaveBtn: "editor.save",
    editorCloseBtn: "action.close",
    deployLogTitle: "deploy.logs.title",
    deployLogHint: "deploy.logs.waiting",
    deployLogCloseBtn: "action.close",
    qqTitle: "qq.modal.title",
    qqCloseBtn: "action.close",
    qqStatusTitle: "qq.modal.step.status",
    qqInstallTitle: "qq.modal.step.install",
    qqUrlLabel: "qq.modal.url.label",
    qqOverwriteText: "qq.modal.overwrite",
    qqInstallBtn: "qq.modal.install",
    qqRefreshBtn: "action.refresh",
    qqLogsTitle: "qq.modal.step.logs",
    qrTitle: "detail.qr.title",
    qrCloseBtn: "action.close",
    confirmInputLabel: "confirm.input_label",
  };
  const PLACEHOLDERS = {
    initPassword: "auth.init.placeholder",
    loginPassword: "auth.login.placeholder",
    serviceSearch: "service.search.placeholder",
    createRegistry: "create.registry.placeholder",
    createDisplay: "create.display.placeholder",
    detailDisplayName: "create.display.placeholder",
    detailOpenPathUrl: "detail.open_path.custom.placeholder",
  };

  let texts = {};
  let observer = null;
  let observerTimer = 0;
  let applyingDynamicText = false;
  let routeSyncBlocked = false;
  let booted = false;
  let collapsed = false;

  function $(id) {
    return document.getElementById(id);
  }

  function text(key, fallback) {
    const value = texts[key];
    if (typeof value === "string" && value.length) {
      return value;
    }
    return fallback || "";
  }

  window.text = text;
  window.textReady = loadTexts();
  async function loadTexts() {
    try {
      const res = await fetch(appPath("/texts.zh-CN.json"), {
        credentials: "same-origin",
      });
      if (!res.ok) {
        throw new Error("failed to load texts");
      }
      texts = await res.json();
    } catch (_) {
      texts = {};
    }
  }

  function applyTextContent() {
    Object.entries(TEXT_IDS).forEach(([id, key]) => {
      const node = $(id);
      if (node && key !== "confirmInputLabel") {
        node.textContent = text(key, node.textContent || "");
      }
    });
    Object.entries(PLACEHOLDERS).forEach(([id, key]) => {
      const node = $(id);
      if (node) {
        node.placeholder = text(key, node.placeholder || "");
      }
    });
    document.title = text("app.title", document.title);
    const fileNameHead = $("fileNameHead");
    const fileTypeHead = $("fileTypeHead");
    const fileSizeHead = $("fileSizeHead");
    const fileMtimeHead = $("fileMtimeHead");
    if (fileNameHead)
      fileNameHead.textContent = text(
        "file.table.name",
        fileNameHead.textContent || "",
      );
    if (fileTypeHead)
      fileTypeHead.textContent = text(
        "file.table.type",
        fileTypeHead.textContent || "",
      );
    if (fileSizeHead)
      fileSizeHead.textContent = text(
        "file.table.size",
        fileSizeHead.textContent || "",
      );
    if (fileMtimeHead)
      fileMtimeHead.textContent = text(
        "file.table.mtime",
        fileMtimeHead.textContent || "",
      );
  }

  function applyDynamicText() {
    if (applyingDynamicText) {
      return;
    }
    applyingDynamicText = true;
    try {
      document.querySelectorAll(".tab-btn[data-tab]").forEach((btn) => {
        const key = "detail.tab." + btn.dataset.tab;
        btn.textContent = text(key, btn.textContent || "");
      });
      const detailConfigBtn = $("detailConfigBtn");
      if (detailConfigBtn)
        detailConfigBtn.textContent = text(
          "detail.config_center",
          detailConfigBtn.textContent || "",
        );
      const detailForceBtn = $("detailForceBtn");
      if (detailForceBtn)
        detailForceBtn.textContent = text(
          "detail.force_stop",
          detailForceBtn.textContent || "",
        );
      const detailMoreBtn = $("detailMoreBtn");
      if (detailMoreBtn)
        detailMoreBtn.textContent = text(
          "action.more",
          detailMoreBtn.textContent || "",
        );
      const servicesCreateBtn = $("servicesCreateBtn");
      if (servicesCreateBtn)
        servicesCreateBtn.textContent = text(
          "service.create",
          servicesCreateBtn.textContent || "",
        );
      document
        .querySelectorAll(".service-card-actions .btn")
        .forEach((node) => {
          node.textContent = text(
            "service.enter_manage",
            node.textContent || "",
          );
        });
      const detailEntryBtn = $("detailEntryBtn");
      const overviewTypeValue = $("overviewTypeValue");
      if (detailEntryBtn && overviewTypeValue) {
        const isLagrange =
          (overviewTypeValue.textContent || "").trim() === "Lagrange";
        detailEntryBtn.textContent = isLagrange
          ? text("detail.show_qr", detailEntryBtn.textContent || "")
          : text("detail.open_path", detailEntryBtn.textContent || "");
      }
      const createAutoStart = $("createAutoStart");
      if (createAutoStart && createAutoStart.options.length >= 2) {
        createAutoStart.options[0].text = text(
          "common.no",
          createAutoStart.options[0].text,
        );
        createAutoStart.options[1].text = text(
          "common.yes",
          createAutoStart.options[1].text,
        );
      }
      const createType = $("createType");
      if (createType && createType.options.length >= 3) {
        createType.options[0].text = text(
          "create.type.sealdice",
          createType.options[0].text,
        );
        createType.options[1].text = text(
          "create.type.lagrange",
          createType.options[1].text,
        );
        createType.options[2].text = text(
          "create.type.llbot",
          createType.options[2].text,
        );
      }
    } finally {
      applyingDynamicText = false;
    }
  }

  function scheduleDynamicTextApply() {
    if (observerTimer) {
      return;
    }
    observerTimer = window.setTimeout(() => {
      observerTimer = 0;
      if (observer) {
        observer.disconnect();
      }
      try {
        applyDynamicText();
      } finally {
        if (observer) {
          observer.observe(document.body, {
            childList: true,
            subtree: true,
          });
        }
      }
    }, 0);
  }

  function observeDynamicText() {
    if (observer) {
      observer.disconnect();
    }
    observer = new MutationObserver(() => {
      scheduleDynamicTextApply();
    });
    observer.observe(document.body, {
      childList: true,
      subtree: true,
    });
  }

  function computeBasePrefix() {
    const parts = window.location.pathname.split("/").filter(Boolean);
    const routeIndex = parts.findIndex((part) => ROUTE_ROOTS.has(part));
    if (routeIndex >= 0) {
      return "/" + parts.slice(0, routeIndex).join("/");
    }
    if (parts.length === 1 && !parts[0].includes(".")) {
      return "/" + parts[0];
    }
    return "";
  }

  function appPath(relativePath) {
    const base = computeBasePrefix();
    if (base && base !== "/") {
      return base + relativePath;
    }
    return relativePath;
  }

  function currentServiceId() {
    const crumb = $("detailBreadcrumb");
    if (!crumb) {
      return "";
    }
    const parts = (crumb.textContent || "").split("/");
    return (parts[parts.length - 1] || "").trim();
  }

  function isDetailVisible() {
    const detail = $("serviceDetailView");
    return !!detail && !detail.classList.contains("hidden");
  }

  function pushRoute(path, replace) {
    const target = appPath(path);
    if (window.location.pathname === target) {
      return;
    }
    routeSyncBlocked = true;
    if (replace) {
      window.history.replaceState({}, "", target);
    } else {
      window.history.pushState({}, "", target);
    }
    window.setTimeout(() => {
      routeSyncBlocked = false;
    }, 0);
  }

  function syncRouteFromUI() {
    if (routeSyncBlocked) {
      return;
    }
    if ($("authInit") && !$("authInit").classList.contains("hidden")) {
      return;
    }
    if ($("authLogin") && !$("authLogin").classList.contains("hidden")) {
      return;
    }
    const detailId = currentServiceId();
    if (isDetailVisible() && detailId) {
      const activeTab = document.querySelector(".tab-btn.active");
      const tab =
        activeTab && SERVICE_TABS.has(activeTab.dataset.tab)
          ? activeTab.dataset.tab
          : "overview";
      pushRoute("/services/" + encodeURIComponent(detailId) + "/" + tab, false);
      return;
    }
    const activeNav = document.querySelector(".nav-btn.active");
    if (activeNav && activeNav.id === "navSettings") {
      pushRoute("/settings", false);
      return;
    }
    if (activeNav && activeNav.id === "navServices") {
      pushRoute("/services", false);
      return;
    }
    pushRoute("/dashboard", false);
  }

  function clickWhenAvailable(selector, done, retries) {
    const node = document.querySelector(selector);
    if (node) {
      node.click();
      if (typeof done === "function") {
        window.setTimeout(done, 30);
      }
      return;
    }
    if (retries <= 0) {
      return;
    }
    window.setTimeout(
      () => clickWhenAvailable(selector, done, retries - 1),
      120,
    );
  }

  function activateRoute() {
    if (!booted) {
      return;
    }
    const rawParts = window.location.pathname.split("/").filter(Boolean);
    let parts = rawParts.slice();
    const routeIndex = parts.findIndex((part) => ROUTE_ROOTS.has(part));
    if (routeIndex > 0) {
      parts = parts.slice(routeIndex);
    }
    const legacyService = new URL(window.location.href).searchParams.get(
      "service",
    );
    if (!parts.length && legacyService) {
      pushRoute(
        "/services/" + encodeURIComponent(legacyService) + "/overview",
        true,
      );
      parts = ["services", legacyService, "overview"];
    }
    if (!parts.length) {
      pushRoute("/dashboard", true);
      parts = ["dashboard"];
    }
    if (parts[0] === "dashboard") {
      $("navDashboard") && $("navDashboard").click();
      return;
    }
    if (parts[0] === "settings") {
      $("navSettings") && $("navSettings").click();
      return;
    }
    if (parts[0] === "services") {
      $("navServices") && $("navServices").click();
      if (parts.length === 1) {
        const backBtn = $("detailBackBtn");
        if (backBtn && isDetailVisible()) {
          window.setTimeout(() => backBtn.click(), 30);
        }
        return;
      }
      const serviceId = decodeURIComponent(parts[1] || "");
      const tab = SERVICE_TABS.has(parts[2]) ? parts[2] : "overview";
      clickWhenAvailable(
        '[data-open="' +
          cssEscape(serviceId) +
          '"], [data-sv="' +
          cssEscape(serviceId) +
          '"]',
        function () {
          clickWhenAvailable(
            '.tab-btn[data-tab="' + cssEscape(tab) + '"]',
            null,
            10,
          );
        },
        20,
      );
    }
  }

  function cssEscape(value) {
    if (window.CSS && typeof window.CSS.escape === "function") {
      return window.CSS.escape(value);
    }
    return String(value).replace(/["\\]/g, "\\$&");
  }

  function bindRouteHandlers() {
    const navDashboard = $("navDashboard");
    const navSettings = $("navSettings");
    const navServices = $("navServices");
    const detailBackBtn = $("detailBackBtn");
    const dashboardToServices = $("dashboardToServices");

    if (navDashboard) {
      navDashboard.addEventListener("click", () =>
        window.setTimeout(() => pushRoute("/dashboard", false), 0),
      );
    }
    if (navSettings) {
      navSettings.addEventListener("click", () =>
        window.setTimeout(() => pushRoute("/settings", false), 0),
      );
    }
    if (navServices) {
      navServices.addEventListener("click", () =>
        window.setTimeout(() => pushRoute("/services", false), 0),
      );
    }
    if (detailBackBtn) {
      detailBackBtn.addEventListener("click", () =>
        window.setTimeout(() => pushRoute("/services", false), 0),
      );
    }
    if (dashboardToServices) {
      dashboardToServices.addEventListener("click", () =>
        window.setTimeout(() => pushRoute("/services", false), 0),
      );
    }

    document.addEventListener("click", (event) => {
      const serviceTrigger = event.target.closest("[data-open], [data-sv]");
      if (serviceTrigger) {
        const serviceId =
          serviceTrigger.getAttribute("data-open") ||
          serviceTrigger.getAttribute("data-sv");
        if (serviceId) {
          window.setTimeout(
            () =>
              pushRoute(
                "/services/" + encodeURIComponent(serviceId) + "/overview",
                false,
              ),
            0,
          );
        }
        return;
      }
      const tabTrigger = event.target.closest(".tab-btn[data-tab]");
      if (tabTrigger) {
        const serviceId = currentServiceId();
        const tab = tabTrigger.dataset.tab;
        if (serviceId && SERVICE_TABS.has(tab)) {
          window.setTimeout(
            () =>
              pushRoute(
                "/services/" + encodeURIComponent(serviceId) + "/" + tab,
                false,
              ),
            0,
          );
        }
      }
    });

    window.addEventListener("popstate", activateRoute);
  }

  function syncSidebarButton() {
    const btn = $("sidebarFloatToggle");
    const appShell = $("appShell");
    if (!btn || !appShell) {
      return;
    }
    const mobile = window.innerWidth <= 1100;
    const isClosed = mobile
      ? !appShell.classList.contains("sidebar-mobile-open")
      : appShell.classList.contains("sidebar-collapsed");
    btn.classList.toggle("is-collapsed", isClosed);
    btn.title = text("nav.sidebar_toggle", "");
  }

  function setSidebarCollapsed(next) {
    const appShell = $("appShell");
    if (!appShell) {
      return;
    }
    collapsed = !!next;
    appShell.classList.toggle(
      "sidebar-collapsed",
      collapsed && window.innerWidth > 1100,
    );
    appShell.classList.toggle(
      "sidebar-mobile-open",
      !collapsed && window.innerWidth <= 1100,
    );
    try {
      window.localStorage.setItem(
        "ldm.sidebar.collapsed",
        collapsed ? "1" : "0",
      );
    } catch (_) {
      // ignore storage errors
    }
    syncSidebarButton();
  }

  function toggleSidebar() {
    setSidebarCollapsed(!collapsed);
  }

  function bindSidebarToggle() {
    const btn = $("sidebarFloatToggle");
    try {
      collapsed = window.localStorage.getItem("ldm.sidebar.collapsed") === "1";
    } catch (_) {
      collapsed = false;
    }
    setSidebarCollapsed(collapsed);
    if (btn) {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        toggleSidebar();
      });
      btn.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          toggleSidebar();
        }
      });
    }
    window.addEventListener("resize", () => {
      setSidebarCollapsed(collapsed);
      syncSidebarButton();
    });
  }

  function enhanceSidebar() {
    const navServicesToggle = $("navServicesToggle");
    const wrap = $("serviceTreeWrap");
    if (!navServicesToggle || !wrap) {
      return;
    }
    const sync = () => {
      navServicesToggle.classList.toggle(
        "is-open",
        !wrap.classList.contains("hidden"),
      );
    };
    sync();
    const mo = new MutationObserver(sync);
    mo.observe(wrap, { attributes: true, attributeFilter: ["class"] });
  }

  function markBooted() {
    const appShell = $("appShell");
    if (!appShell || appShell.classList.contains("hidden")) {
      window.setTimeout(markBooted, 150);
      return;
    }
    booted = true;
    activateRoute();
  }

  async function boot() {
    await window.textReady;
    applyTextContent();
    applyDynamicText();
    observeDynamicText();
    bindRouteHandlers();
    enhanceSidebar();
    bindSidebarToggle();
    markBooted();
  }

  document.addEventListener("DOMContentLoaded", boot);
})();
