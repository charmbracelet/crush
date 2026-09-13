// Counts the tab down and then asks the browser to close it.
//
// Whether the browser obeys is out of our hands. A tab opened by Crush
// rather than by script may only close itself while its session history
// holds a single entry, so a plain redirect chain can close but a consent
// screen the user had to click through usually cannot. Treat closing as a
// request that may well be refused and always leave a readable message
// behind. A failed authorization carries no delay at all: its message is
// rendered server-side and left alone for the reader.
//
// The handoff page skips the countdown; its job is the continue button.
// Clicking it opens the authorization URL in a new tab that keeps this
// page as its opener — the one arrangement browsers let close itself
// after a consent flow — and then this tab tries to follow it out.
(function () {
  const rail = document.getElementById("rail");
  const status = document.getElementById("status");
  const continueLink = document.getElementById("continue");
  if (!status) return;

  if (continueLink) {
    continueLink.addEventListener("click", () => {
      // Hand the tab to the authorization page first; closing this one
      // does not disturb it.
      window.setTimeout(() => {
        status.textContent = "You can close this tab.";
        window.close();
      }, 250);
    });
    return;
  }

  if (!rail) return;

  const delay = parseInt(rail.dataset.delay, 10);
  if (!Number.isFinite(delay) || delay <= 0) return;

  let left = delay;

  const render = () => {
    status.innerHTML =
      'Closing in <span class="count">' +
      left +
      "</span> " +
      (left === 1 ? "second" : "seconds") +
      "…";
  };

  const tick = () => {
    left -= 1;
    if (left > 0) {
      render();
      return;
    }
    clearInterval(timer);
    window.close();
    // Still running, so the browser refused. Say so rather than leaving a
    // countdown frozen at zero.
    setTimeout(() => {
      status.textContent = "You can close this tab.";
    }, 250);
  };

  render();
  rail.style.setProperty("--delay", delay + "s");
  rail.classList.add("running");
  const timer = setInterval(tick, 1000);
})();
