(function () {
  const INSTANCE_KEY = "__slWidgetPostTtsHook";
  const POST_URL = "http://127.0.0.1:8787/tts";

  if (window[INSTANCE_KEY]) {
    try {
      window[INSTANCE_KEY].cleanup();
    } catch (_) {}
  }

  let lastMessage = null;
  let lastWasHidden = null;
  let cleanedUp = false;

  const observers = [];
  const timers = [];

  window[INSTANCE_KEY] = { cleanup };

  function cleanup() {
    if (cleanedUp) return;
    cleanedUp = true;

    observers.forEach(o => o.disconnect());
    timers.forEach(t => clearInterval(t));
    observers.length = 0;
    timers.length = 0;
  }

  function cleanText(value) {
    const text = (value || "").replace(/\s+/g, " ").trim();
    return text || null;
  }

  function captureMessage() {
    if (cleanedUp) return;

    const el = document.getElementById("alert-user-message");
    const text = cleanText(el?.textContent || el?.innerText);

    if (text) {
      lastMessage = text;
    }
  }

  function postMessage(text) {
    const message = cleanText(text);

    if (!message) return;

    fetch(POST_URL, {
      method: "POST",
      mode: "no-cors",
      headers: {
        "Content-Type": "text/plain"
      },
      body: message
    });
  }

  function later(fn, delay) {
    const id = setTimeout(() => {
      if (!cleanedUp) fn();
    }, delay);

    timers.push(id);
  }

  function setupMessageObserver() {
    const el = document.getElementById("alert-user-message");
    if (!el) return false;

    captureMessage();

    const observer = new MutationObserver(captureMessage);

    observer.observe(el, {
      childList: true,
      characterData: true,
      subtree: true
    });

    observers.push(observer);
    return true;
  }

  function setupWidgetObserver() {
    const widget = document.getElementById("widget");
    if (!widget) return false;

    lastWasHidden = widget.classList.contains("hidden");

    const observer = new MutationObserver(() => {
      if (cleanedUp) return;

      const isHidden = widget.classList.contains("hidden");

      // Alert started
      if (lastWasHidden && !isHidden) {
        lastMessage = null;

        later(captureMessage, 50);
        later(captureMessage, 250);
        later(captureMessage, 750);
      }

      // Alert finished
      if (!lastWasHidden && isHidden) {
        postMessage(lastMessage);
        lastMessage = null;
      }

      lastWasHidden = isHidden;
    });

    observer.observe(widget, {
      attributes: true,
      attributeFilter: ["class"]
    });

    observers.push(observer);
    return true;
  }

  function waitFor(setup) {
    if (setup()) return;

    const id = setInterval(() => {
      if (cleanedUp || setup()) {
        clearInterval(id);
      }
    }, 250);

    timers.push(id);
  }

  waitFor(setupMessageObserver);
  waitFor(setupWidgetObserver);
})();