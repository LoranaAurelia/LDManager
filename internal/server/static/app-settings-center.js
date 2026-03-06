(() => {
  "use strict";

  /**
   * 服务配置中心模块。
   *
   * 目标：
   * 1. 将设置中心相关逻辑从 app.js 中拆分出来，避免主文件继续膨胀。
   * 2. 保持现有行为一致，只优化可读性、维护性与边界处理。
   * 3. 集中管理“运行中服务禁止改端口/协议”的交互逻辑。
   */
  function create(deps) {
    const {
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
      showWarning,
      toast,
    } = deps;

    function normalizeServers(servers) {
      return (servers || [])
        .map((v) =>
          typeof v === "string"
            ? { name: v, url: v, selected: false }
            : {
                name: v.name || v.url || "",
                url: v.url || v.name || "",
                selected: !!v.selected,
                base_name: v.base_name || v.name || v.url || "",
              },
        )
        .filter((v) => !!v.url);
    }

    function findSignItemByVersion(version) {
      return (S.sign || []).find(
        (item, index) => (item.version || String(index)) === version,
      );
    }

    async function probeAllSignServers(version, selectID, textID) {
      const hit = findSignItemByVersion(version);
      if (!hit) return;
      const normalized = normalizeServers(hit.servers);
      if (!normalized.length) return;
      const out = $(textID);
      if (out) out.textContent = tx("create.lagrange.probing");
      let ok = 0;
      await Promise.all(
        normalized.map(async (srv) => {
          const baseName = srv.base_name || srv.name || srv.url;
          const r = await probeSignLatency(srv.url, 5);
          srv.base_name = baseName;
          srv.selected = !!srv.selected;
          srv.name = r.ok
            ? `${baseName} (${r.avg_ms}ms)`
            : `${baseName} (${tx("create.lagrange.failed_short")})`;
          if (r.ok) ok++;
        }),
      );
      hit.servers = normalized;
      const sel = $(selectID);
      const keep = sel ? sel.value : "";
      if (sel) {
        sel.innerHTML = srvOpts(version, keep);
      }
      if (out) {
        out.textContent = tx("create.lagrange.probe_done")
          .replace("{ok}", String(ok))
          .replace("{total}", String(normalized.length));
      }
      probeCurrentSign(selectID, textID);
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

    /**
     * 渲染 Lagrange 端口/实现卡片 + 签名卡片。
     * `locked=true` 表示服务正在运行，所有会影响热更新稳定性的配置统一置灰锁定。
     */
    function lagForm(config, target = "portsContent", locked = false) {
      const currentSignURL = String(config.sign_server_url || "");
      const signSelected = signOpts(currentSignURL);
      const signKnown = (S.sign || []).some((item) =>
        normalizeServers(item.servers).some((srv) => srv.url === currentSignURL),
      );
      const signVersionValue = signKnown ? signSelected.ver : "custom";
      const lockCardClass = locked ? " config-card-locked" : "";

      E[target].innerHTML = `
        <div class="stack-block card-block config-card${lockCardClass}">
          <h3>${tx("detail.lagrange.config.title")}</h3>
          <div class="impl-grid">
            <div class="impl-card">
              <label class="checkbox-row checkbox-row-switch">
                <input id="cfgEnableForward" class="force-on-switch" type="checkbox" checked disabled>
                <span>${tx("create.lagrange.impl.forward.required")}</span>
              </label>
              <div class="impl-port-wrap">
                <label>${tx("create.port.label")}</label>
                <input id="cfgForwardPort" type="number" min="1" max="65535"
                  value="${esc(config.forward_ws_port || 0)}" ${locked ? "disabled" : ""}>
                <div id="cfgForwardPortHint" class="port-hint"></div>
              </div>
            </div>

            <div class="impl-card">
              <label class="checkbox-row checkbox-row-switch">
                <input id="cfgEnableReverse" type="checkbox"
                  ${config.enable_reverse_ws ? "checked" : ""} ${locked ? "disabled" : ""}>
                <span>${tx("create.lagrange.impl.reverse")}</span>
              </label>
              <div id="cfgReverseWrap"
                class="impl-port-wrap ${config.enable_reverse_ws ? "" : "hidden"}">
                <label>${tx("create.port.label")}</label>
                <input id="cfgReversePort" type="number" min="1" max="65535"
                  value="${esc(config.reverse_ws_port || 0)}" ${locked ? "disabled" : ""}>
                <div id="cfgReversePortHint" class="port-hint"></div>
              </div>
            </div>

            <div class="impl-card">
              <label class="checkbox-row checkbox-row-switch">
                <input id="cfgEnableHTTP" type="checkbox"
                  ${config.enable_http ? "checked" : ""} ${locked ? "disabled" : ""}>
                <span>${tx("create.lagrange.impl.http")}</span>
              </label>
              <div id="cfgHTTPWrap" class="impl-port-wrap ${config.enable_http ? "" : "hidden"}">
                <label>${tx("create.port.label")}</label>
                <input id="cfgHTTPPort" type="number" min="1" max="65535"
                  value="${esc(config.http_port || 0)}" ${locked ? "disabled" : ""}>
                <div id="cfgHTTPPortHint" class="port-hint"></div>
              </div>
            </div>
          </div>
          ${
            locked
              ? `<div class="config-lock-note config-lock-warn">${tx("detail.settings.locked_ports")}</div>`
              : ""
          }
        </div>

        <div class="stack-block card-block config-card${lockCardClass}">
          <h3>${tx("create.lagrange.step.sign")}</h3>
          <div class="form-grid sign-grid">
            <div>
              <label>${tx("create.lagrange.sign.version.label")}</label>
              <select id="cfgSignVersion" ${locked ? "disabled" : ""}>
                ${(S.sign || [])
                  .map((item, index) => {
                    const version = item.version || String(index);
                    const selected =
                      version === signVersionValue ? "selected" : "";
                    return `<option value="${esc(version)}" ${selected}>${esc(version)}</option>`;
                  })
                  .join("")}
                <option value="custom" ${signVersionValue === "custom" ? "selected" : ""}>${tx("create.sign.custom", "自定义")}</option>
              </select>
            </div>
            <div id="cfgSignServerSelectWrap">
              <label>${tx("create.lagrange.sign.label")}</label>
              <select id="cfgSignServer" ${locked ? "disabled" : ""}>
                ${srvOpts(signSelected.ver, signSelected.srv)}
              </select>
            </div>
            <div id="cfgSignServerCustomWrap" class="hidden">
              <label>${tx("create.lagrange.sign.custom_url", "自定义签名地址")}</label>
              <input id="cfgSignCustom" type="text" value="${esc(currentSignURL)}" ${locked ? "disabled" : ""} placeholder="${tx("create.lagrange.sign.custom_placeholder", "请输入签名服务地址")}">
              <div id="cfgSignLatency" class="muted inline-latency"></div>
            </div>
            <div class="config-span-2">
              <div class="inline-tools">
                <button id="cfgSignProbeBtn" class="btn btn-soft" type="button" ${locked ? "disabled" : ""}>${tx("create.lagrange.sign.test_all")}</button>
                <button id="cfgSignProbeCustomBtn" class="btn btn-soft hidden" type="button" ${locked ? "disabled" : ""}>${tx("create.lagrange.sign.test_one", "测试地址")}</button>
              </div>
            </div>
          </div>
        </div>
      `;

      const isCustomSign = () => $("cfgSignVersion").value === "custom";
      const getConfigSignURL = () =>
        isCustomSign()
          ? String(($("cfgSignCustom") && $("cfgSignCustom").value) || "").trim()
          : String(($("cfgSignServer") && $("cfgSignServer").value) || "").trim();
      const updateSignMode = () => {
        const custom = isCustomSign();
        $("cfgSignServerSelectWrap").classList.toggle("hidden", custom);
        $("cfgSignServerCustomWrap").classList.toggle("hidden", !custom);
        $("cfgSignProbeBtn").classList.toggle("hidden", custom);
        $("cfgSignProbeCustomBtn").classList.toggle("hidden", !custom);
      };
      const refreshSignServers = async () => {
        updateSignMode();
        if (isCustomSign()) {
          $("cfgSignLatency").textContent = "";
          return;
        }
        const ver = $("cfgSignVersion").value;
        $("cfgSignServer").innerHTML = srvOpts(ver, "");
        await probeAllSignServers(ver, "cfgSignServer", "cfgSignLatency");
      };
      $("cfgSignVersion").onchange = refreshSignServers;

      $("cfgEnableReverse").onchange = () => {
        $("cfgReverseWrap").classList.toggle(
          "hidden",
          !$("cfgEnableReverse").checked,
        );
        if (!$("cfgEnableReverse").checked) {
          setPortHint($("cfgReversePort"), $("cfgReversePortHint"), "", false);
        }
      };

      $("cfgEnableHTTP").onchange = () => {
        $("cfgHTTPWrap").classList.toggle(
          "hidden",
          !$("cfgEnableHTTP").checked,
        );
        if (!$("cfgEnableHTTP").checked) {
          setPortHint($("cfgHTTPPort"), $("cfgHTTPPortHint"), "", false);
        }
      };

      $("cfgSignServer").onchange = () =>
        probeCurrentSign("cfgSignServer", "cfgSignLatency");
      if ($("cfgSignCustom")) {
        $("cfgSignCustom").oninput = () => {
          $("cfgSignLatency").textContent = "";
        };
      }
      if ($("cfgSignProbeBtn")) {
        $("cfgSignProbeBtn").onclick = async () => {
          const ver = $("cfgSignVersion") ? $("cfgSignVersion").value : "";
          if (!ver) return;
          await probeAllSignServers(ver, "cfgSignServer", "cfgSignLatency");
        };
      }
      if ($("cfgSignProbeCustomBtn")) {
        $("cfgSignProbeCustomBtn").onclick = async () => {
          const url = getConfigSignURL();
          if (!url) {
            $("cfgSignLatency").textContent = tx("create.lagrange.no_servers");
            return;
          }
          $("cfgSignLatency").textContent = tx("create.lagrange.ping_checking");
          const r = await probeSignLatency(url, 5);
          $("cfgSignLatency").textContent = r.ok
            ? tx("create.lagrange.ping_ok").replace("{avg}", r.avg_ms)
            : tx("create.lagrange.ping_fail");
        };
      }

      if (!locked) {
        const serviceID = S.svc ? S.svc.id : "";
        bindPortFieldRealtime(
          "cfgForwardPort",
          "cfgForwardPortHint",
          serviceID,
        );
        bindPortFieldRealtime(
          "cfgReversePort",
          "cfgReversePortHint",
          serviceID,
        );
        bindPortFieldRealtime("cfgHTTPPort", "cfgHTTPPortHint", serviceID);
      }

      setTimeout(async () => {
        updateSignMode();
        const ver = $("cfgSignVersion") ? $("cfgSignVersion").value : "";
        if (ver && ver !== "custom") {
          await probeAllSignServers(ver, "cfgSignServer", "cfgSignLatency");
        }
      }, 30);
    }

    /**
     * 渲染单端口卡片（Sealdice/LLBot 使用）。
     */
    function portForm(port, target = "portsContent", locked = false) {
      const isWebUISvc =
        S.svc && (S.svc.type === "Sealdice" || S.svc.type === "LuckyLilliaBot");
      const title = isWebUISvc ? tx("detail.edit_webui_port") : tx("detail.edit_port");
      const portLabel = isWebUISvc
        ? tx("create.webui_port.label")
        : tx("create.port.label");
      const lockCardClass = locked ? " config-card-locked" : "";

      E[target].innerHTML = `
        <div class="stack-block card-block config-card${lockCardClass}">
          <h3>${title}</h3>
          <div class="single-port-card">
            <div>
              <label>${portLabel}</label>
              <input id="portSingleInput" type="number" min="1" max="65535"
                value="${esc(port || "")}" ${locked ? "disabled" : ""}>
              <div id="portSingleInputHint" class="port-hint"></div>
            </div>
          </div>
          ${
            locked
              ? `<div class="config-lock-note config-lock-warn">${tx("detail.settings.locked_port_single")}</div>`
              : ""
          }
        </div>
      `;

      if (!locked) {
        bindPortFieldRealtime(
          "portSingleInput",
          "portSingleInputHint",
          S.svc ? S.svc.id : "",
        );
      }
    }

    /**
     * 打开配置中心弹窗并加载当前服务配置。
     */
    async function setOpen() {
      if (!S.svc) return;
      try {
        E.settingsDrawerTitle.textContent = tx("detail.config_center");

        const settings = await j(
          `services/${encodeURIComponent(S.svc.id)}/settings`,
        );
        E.detailDisplayName.value = settings.display_name || "";
        E.detailAutoStart.checked = !!settings.auto_start;
        E.detailOpenPathUrl.value = settings.open_path_url || "";
        E.detailRestartEnabled.checked = !!(
          settings.restart && settings.restart.enabled
        );
        E.detailRestartDelay.value =
          (settings.restart && settings.restart.delay_seconds) || 0;
        E.detailRestartMax.value =
          (settings.restart && settings.restart.max_crash_count) || 0;
        if (E.detailLogRetention)
          E.detailLogRetention.value =
            (settings.log_policy && settings.log_policy.retention_count) || 0;
        if (E.detailLogRetentionDays)
          E.detailLogRetentionDays.value =
            (settings.log_policy && settings.log_policy.retention_days) || 0;
        if (E.detailLogMaxMB)
          E.detailLogMaxMB.value =
            (settings.log_policy && settings.log_policy.max_mb) || 0;
        E.detailRestartCurrent.textContent = tx(
          "restart.current_crash",
        ).replace(
          "{count}",
          (settings.restart && settings.restart.consecutive_crash) || 0,
        );

        if (S.svc.type === "Lagrange") hide(E.openPathWrap);
        else show(E.openPathWrap);

        S.settingsSnapshot = { port: Number(S.svc.port || 0), lagrange: null };
        const locked = S.svc.status === "running";

        if (S.svc.type === "Lagrange") {
          const config = await j(
            `services/${encodeURIComponent(S.svc.id)}/config`,
          );
          S.settingsSnapshot.lagrange = {
            enable_forward_ws: true,
            forward_ws_port: Number(config.forward_ws_port || 0),
            enable_reverse_ws: !!config.enable_reverse_ws,
            reverse_ws_port: Number(config.reverse_ws_port || 0),
            enable_http: !!config.enable_http,
            http_port: Number(config.http_port || 0),
            sign_server_url: config.sign_server_url || "",
          };
          lagForm(config, "settingsPortSection", locked);
        } else {
          portForm(S.svc.port || "", "settingsPortSection", locked);
        }

        E.settingsActionSection.innerHTML =
          S.svc.type === "LuckyLilliaBot"
            ? `<div class="stack-block card-block config-card">
                <h3>${tx("detail.manage_qq")}</h3>
                <div class="inline-tools">
                  <button id="settingsQQBtn" class="btn btn-soft" type="button">
                    ${tx("detail.manage_qq")}
                  </button>
                </div>
              </div>`
            : "";

        if ($("settingsQQBtn")) {
          $("settingsQQBtn").onclick = () => qqOpen(false);
        }
        show(E.settingsDrawer);
      } catch (err) {
        toast(err.message, "error");
      }
    }

    /**
     * 保存设置中心内容。
     *
     * 保存策略：
     * 1. 始终先保存“运行中可安全修改”的通用设置（显示名、自启、自动重启、访问地址）。
     * 2. 仅在服务停止时，才允许保存端口/协议/签名等配置。
     * 3. 仅在对应字段确实发生变化时才做端口占用校验，避免误报。
     */
    async function setSave() {
      if (!S.svc) return;
      try {
        await j(`services/${encodeURIComponent(S.svc.id)}/settings`, {
          method: "POST",
          body: JSON.stringify({
            display_name: E.detailDisplayName.value.trim(),
            auto_start: E.detailAutoStart.checked,
            open_path_url: E.detailOpenPathUrl.value.trim(),
            restart: {
              enabled: E.detailRestartEnabled.checked,
              delay_seconds: Number(E.detailRestartDelay.value || 0),
              max_crash_count: Number(E.detailRestartMax.value || 0),
            },
            log_policy: {
              retention_count: Number(
                (E.detailLogRetention && E.detailLogRetention.value) || 0,
              ),
              retention_days: Number(
                (E.detailLogRetentionDays && E.detailLogRetentionDays.value) ||
                  0,
              ),
              max_mb: Number((E.detailLogMaxMB && E.detailLogMaxMB.value) || 0),
            },
          }),
        });

        const allowPortConfig = S.svc.status !== "running";

        if (allowPortConfig && S.svc.type === "Lagrange") {
          const oldConfig =
            (S.settingsSnapshot && S.settingsSnapshot.lagrange) || null;
          const desiredForward = Number($("cfgForwardPort").value || 0);
          const desiredEnableReverse = !!$("cfgEnableReverse").checked;
          const desiredReverse = desiredEnableReverse
            ? Number($("cfgReversePort").value || 0)
            : 0;
          const desiredEnableHTTP = !!$("cfgEnableHTTP").checked;
          const desiredHTTP = desiredEnableHTTP
            ? Number($("cfgHTTPPort").value || 0)
            : 0;
          const desiredSign =
            String(
              $("cfgSignVersion").value === "custom"
                ? ($("cfgSignCustom") && $("cfgSignCustom").value) || ""
                : ($("cfgSignServer") && $("cfgSignServer").value) || "",
            ).trim();

          const changedForward =
            !oldConfig ||
            Number(oldConfig.forward_ws_port || 0) !== desiredForward;
          const changedReverse =
            !oldConfig ||
            !!oldConfig.enable_reverse_ws !== desiredEnableReverse ||
            (desiredEnableReverse &&
              Number(oldConfig.reverse_ws_port || 0) !== desiredReverse);
          const changedHTTP =
            !oldConfig ||
            !!oldConfig.enable_http !== desiredEnableHTTP ||
            (desiredEnableHTTP &&
              Number(oldConfig.http_port || 0) !== desiredHTTP);
          const changedSign =
            !oldConfig ||
            String(oldConfig.sign_server_url || "") !== desiredSign;
          if (changedSign && !desiredSign) {
            throw new Error(tx("create.lagrange.no_servers"));
          }

          const changed =
            changedForward || changedReverse || changedHTTP || changedSign;
          if (changed) {
            let forwardPort = Number(
              (oldConfig && oldConfig.forward_ws_port) || 0,
            );
            let reversePort = Number(
              (oldConfig && oldConfig.reverse_ws_port) || 0,
            );
            let httpPort = Number((oldConfig && oldConfig.http_port) || 0);

            if (changedForward) {
              const checked = await validatePortField(
                "cfgForwardPort",
                "cfgForwardPortHint",
                S.svc.id,
              );
              if (!checked.ok)
                throw new Error(checked.message || tx("error.invalid_port"));
              forwardPort = checked.port;
            }
            if (desiredEnableReverse && changedReverse) {
              const checked = await validatePortField(
                "cfgReversePort",
                "cfgReversePortHint",
                S.svc.id,
              );
              if (!checked.ok)
                throw new Error(checked.message || tx("error.invalid_port"));
              reversePort = checked.port;
            }
            if (desiredEnableHTTP && changedHTTP) {
              const checked = await validatePortField(
                "cfgHTTPPort",
                "cfgHTTPPortHint",
                S.svc.id,
              );
              if (!checked.ok)
                throw new Error(checked.message || tx("error.invalid_port"));
              httpPort = checked.port;
            }

            await showWarning(tx("notice.cloud_port"));
            await j(`services/${encodeURIComponent(S.svc.id)}/config`, {
              method: "POST",
              body: JSON.stringify({
                enable_forward_ws: true,
                forward_ws_port: desiredForward || forwardPort,
                enable_reverse_ws: desiredEnableReverse,
                reverse_ws_port: desiredEnableReverse
                  ? desiredReverse || reversePort
                  : 0,
                enable_http: desiredEnableHTTP,
                http_port: desiredEnableHTTP ? desiredHTTP || httpPort : 0,
                sign_server_url: desiredSign,
              }),
            });
          }
        } else if (allowPortConfig && $("portSingleInput")) {
          const oldPort =
            (S.settingsSnapshot && Number(S.settingsSnapshot.port || 0)) ||
            Number(S.svc.port || 0);
          const newPort = Number($("portSingleInput").value || 0);

          if (newPort !== oldPort) {
            const checked = await validatePortField(
              "portSingleInput",
              "portSingleInputHint",
              S.svc.id,
            );
            if (!checked.ok)
              throw new Error(checked.message || tx("error.invalid_port"));
            await showWarning(tx("notice.cloud_port"));
            await j(`services/${encodeURIComponent(S.svc.id)}/port`, {
              method: "POST",
              body: JSON.stringify({ port: checked.port }),
            });
          }
        }

        hide(E.settingsDrawer);
        await loadSvcs(true);
        toast(tx("detail.settings.saved"), "ok");
      } catch (err) {
        toast(err.message, "error");
      }
    }

    async function portsOpen() {
      return setOpen();
    }

    async function portsSave() {
      return setSave();
    }

    return { setOpen, setSave, portsOpen, portsSave };
  }

  window.SealSettingsCenter = { create };
})();
