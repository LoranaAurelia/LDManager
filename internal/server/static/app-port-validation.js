(() => {
  "use strict";

  function createPortValidation(deps) {
    const { request, tx, getById, doc } = deps;

    function syncPortActionButtons() {
      const hasInvalidCreate = !!doc.querySelector(
        "#createModal:not(.hidden) .input-invalid",
      );
      const hasInvalidSettings = !!doc.querySelector(
        "#settingsDrawer:not(.hidden) .input-invalid",
      );
      if (deps.createSubmitBtn)
        deps.createSubmitBtn.disabled = hasInvalidCreate;
      if (deps.detailSettingsSaveBtn)
        deps.detailSettingsSaveBtn.disabled = hasInvalidSettings;
      if (deps.portsSaveBtn) deps.portsSaveBtn.disabled = hasInvalidSettings;
    }

    function parsePort(value) {
      const n = Number(value);
      if (!Number.isInteger(n) || n < 1 || n > 65535) {
        throw new Error(tx("error.invalid_port"));
      }
      return n;
    }

    async function apiPortCheck(port, serviceID = "") {
      return request("ports/check", {
        method: "POST",
        body: JSON.stringify({
          port: Number(port || 0),
          service_id: serviceID || "",
        }),
      });
    }

    function setPortHint(input, hint, msg, isErr) {
      if (!input) return;
      input.classList.toggle("input-invalid", !!isErr);
      if (hint) {
        hint.textContent = msg || "";
        hint.classList.toggle("error", !!isErr);
      }
      syncPortActionButtons();
    }

    async function validatePortField(inputID, hintID, serviceID = "") {
      const input = getById(inputID);
      const hint = getById(hintID);
      if (!input) return { ok: true };

      const raw = String(input.value || "").trim();
      if (raw === "") {
        setPortHint(input, hint, "", false);
        return { ok: false, message: tx("error.invalid_port") };
      }

      const n = Number(raw);
      if (!Number.isInteger(n) || n < 1 || n > 65535) {
        setPortHint(input, hint, tx("error.invalid_port"), true);
        return { ok: false, message: tx("error.invalid_port") };
      }

      try {
        const data = await apiPortCheck(n, serviceID);
        if (data && data.available) {
          setPortHint(input, hint, tx("create.port.ok"), false);
          return { ok: true, port: n };
        }
        const message = (data && data.message) || tx("create.port.used");
        setPortHint(input, hint, message, true);
        return { ok: false, message };
      } catch (err) {
        const message =
          err && err.message ? err.message : tx("create.port.check_failed");
        setPortHint(input, hint, message, true);
        return { ok: false, message };
      }
    }

    function bindPortFieldRealtime(inputID, hintID, serviceID = "") {
      const input = getById(inputID);
      if (!input) return;
      let timer = 0;
      const run = () => {
        clearTimeout(timer);
        timer = setTimeout(() => {
          validatePortField(inputID, hintID, serviceID);
        }, 180);
      };
      input.addEventListener("input", run);
      input.addEventListener("change", run);
      run();
    }

    return {
      parsePort,
      apiPortCheck,
      setPortHint,
      validatePortField,
      bindPortFieldRealtime,
      syncPortActionButtons,
    };
  }

  window.SealPortValidation = {
    createPortValidation,
  };
})();
