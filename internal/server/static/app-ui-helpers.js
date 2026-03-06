(() => {
  "use strict";

  function syncOverlayState() {
    const appShell = document.getElementById("appShell");
    if (!appShell) return;
    const opened = document.querySelector(".overlay:not(.hidden)");
    appShell.classList.toggle("overlay-open", !!opened);
  }

  function show(el) {
    if (!el) return;
    const animate = !el.classList.contains("overlay");
    el.classList.remove("hidden");
    if (animate) {
      el.classList.remove("ui-enter");
      void el.offsetWidth;
      el.classList.add("ui-enter");
      setTimeout(() => el.classList.remove("ui-enter"), 180);
    } else {
      el.classList.remove("ui-enter");
    }
    syncOverlayState();
  }

  function hide(el) {
    if (!el) return;
    el.classList.add("hidden");
    el.classList.remove("ui-enter");
    syncOverlayState();
  }

  window.SealUIHelpers = {
    syncOverlayState,
    show,
    hide,
  };
})();
