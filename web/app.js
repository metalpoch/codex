const form = document.querySelector("#searchForm");
const input = document.querySelector("#question");
const button = form.querySelector("button[type=submit]");
const answerArea = document.querySelector("#answerArea");
const answerText = document.querySelector("#answerText");
const errorMessage = document.querySelector("#errorMessage");
const sourcesDetails = document.querySelector("#sourcesDetails");
const sourceList = document.querySelector("#sourceList");
const sourceCount = document.querySelector("#sourceCount");
const seekOverlay = document.querySelector("#seekOverlay");
const seekFlip = document.querySelector("#seekFlip");
const seekStack = document.querySelector("#seekStack");
const seekProgress = document.querySelector("#seekProgress");
const seekFall = document.querySelector("#seekFall");
const seekBodyLeft = document.querySelector("#seekBodyLeft");
const seekBodyRight = document.querySelector("#seekBodyRight");
const seekBodyFlip = document.querySelector("#seekBodyFlip");

const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const TOTAL_FOLIOS = 537;

const INK_GLYPHS = ["ᚠ", "ᚢ", "ᚦ", "ᚨ", "ᚱ", "ᚲ", "ᚷ", "ᚹ", "ᚺ", "ᚾ", "ᛁ", "ᛃ", "ᛇ", "ᛈ", "ᛉ", "ᛊ", "ᛏ", "ᛒ", "ᛖ", "ᛗ", "ᛚ", "ᛞ", "ᛟ", "✶", "☽", "✥", "⚚", "☤"];
const SEAL_SVG = '<svg viewBox="0 0 40 40" fill="none"><circle cx="20" cy="20" r="17.2" stroke="currentColor" stroke-width="1.1"/><path d="M20 5.5 26.9 25.8 8.6 13.2h22.8L13.1 25.8Z" stroke="currentColor" stroke-width="1.1" stroke-linejoin="round"/></svg>';

function randomBetween(min, max) {
  return min + Math.random() * (max - min);
}

function randomizePage(body) {
  body.replaceChildren();
  const add = (className, style) => {
    const el = document.createElement("span");
    el.className = className;
    Object.assign(el.style, style);
    body.append(el);
    return el;
  };
  add("seek-rubric", { top: `${randomBetween(9, 14).toFixed(1)}%`, left: "11%", width: `${randomBetween(26, 48).toFixed(1)}%` });
  let top = randomBetween(22, 27);
  while (top < randomBetween(74, 84)) {
    const indent = randomBetween(9, 12);
    const width = top > 68 ? randomBetween(42, 74) : randomBetween(58, 96 - indent);
    add("seek-line", { top: `${top.toFixed(1)}%`, left: `${indent.toFixed(1)}%`, width: `${width.toFixed(1)}%` });
    top += randomBetween(4.4, 6.1);
  }
  if (Math.random() < 0.4) {
    add("seek-ink", {
      top: `${randomBetween(30, 58).toFixed(1)}%`,
      left: `${randomBetween(13, 22).toFixed(1)}%`,
      fontSize: `${randomBetween(11, 15).toFixed(1)}px`,
    }).textContent = INK_GLYPHS[Math.floor(Math.random() * INK_GLYPHS.length)];
  }
  if (Math.random() < 0.18) {
    add("seek-ink", {
      top: `${randomBetween(48, 70).toFixed(1)}%`,
      right: `${randomBetween(12, 20).toFixed(1)}%`,
      fontSize: `${randomBetween(9, 12).toFixed(1)}px`,
    }).textContent = INK_GLYPHS[Math.floor(Math.random() * INK_GLYPHS.length)];
  }
  if (Math.random() < 0.22) {
    const seal = add("seek-seal", {
      top: `${randomBetween(42, 62).toFixed(1)}%`,
      right: `${randomBetween(10, 17).toFixed(1)}%`,
      transform: `rotate(${randomBetween(-28, 28).toFixed(1)}deg)`,
    });
    seal.innerHTML = SEAL_SVG;
  }
  add("seek-folio", {}).textContent = String(1 + Math.floor(Math.random() * TOTAL_FOLIOS));
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const question = input.value.trim();
  if (!question) return;

  button.disabled = true;
  answerArea.hidden = true;
  errorMessage.hidden = true;
  sourcesDetails.open = false;
  openSeek();

  try {
    const response = await fetch("/api/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ question }),
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || "No se pudo consultar el archivo.");

    answerText.innerHTML = renderMarkdown(payload.answer);
    sourceList.replaceChildren();
    const sources = payload.sources || [];
    for (const source of sources) {
      const item = document.createElement("p");
      item.className = "source-excerpt";
      item.append(document.createTextNode(source.excerpt));
      const score = document.createElement("span");
      score.className = "source-score";
      score.textContent = `RELEVANCIA · ${Math.round(source.similarity * 100)}%`;
      item.append(score);
      sourceList.append(item);
    }
    sourceCount.textContent = sources.length ? `· ${sources.length}` : "";
    sourcesDetails.hidden = sources.length === 0;
    answerArea.hidden = false;
    answerArea.scrollIntoView({ behavior: "smooth", block: "nearest" });
  } catch (error) {
    errorMessage.textContent = error.message;
    errorMessage.hidden = false;
  } finally {
    await closeSeek();
    button.disabled = false;
  }
});

document.querySelectorAll("[data-question]").forEach((suggestion) => {
  suggestion.addEventListener("click", () => {
    input.value = suggestion.dataset.question;
    form.requestSubmit();
  });
});

// --- Respuesta en markdown -------------------------------------------------

function escapeHTML(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function renderInline(value) {
  return escapeHTML(value)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/(^|[\s(])\*([^*\n]+)\*/g, "$1<em>$2</em>")
    .replace(/(^|[\s(])_([^_\n]+)_/g, "$1<em>$2</em>");
}

function renderMarkdown(text) {
  const lines = String(text || "").replace(/\r\n?/g, "\n").split("\n");
  const html = [];
  let list = null;
  let paragraph = [];

  const closeList = () => {
    if (!list) return;
    html.push(`</${list}>`);
    list = null;
  };
  const flushParagraph = () => {
    if (!paragraph.length) return;
    closeList();
    html.push(`<p>${paragraph.map(renderInline).join("<br>")}</p>`);
    paragraph = [];
  };
  const openList = (tag) => {
    flushParagraph();
    if (list === tag) return;
    closeList();
    html.push(`<${tag}>`);
    list = tag;
  };

  for (const raw of lines) {
    const line = raw.trim();
    if (!line) {
      flushParagraph();
      continue;
    }
    const heading = line.match(/^#{1,6}\s+(.*)$/);
    if (heading) {
      flushParagraph();
      closeList();
      html.push(`<h3>${renderInline(heading[1])}</h3>`);
      continue;
    }
    const bullet = line.match(/^[-*•▪◦]\s+(.*)$/);
    if (bullet) {
      openList("ul");
      html.push(`<li>${renderInline(bullet[1])}</li>`);
      continue;
    }
    const numbered = line.match(/^\d+[.)]\s+(.*)$/);
    if (numbered) {
      openList("ol");
      html.push(`<li>${renderInline(numbered[1])}</li>`);
      continue;
    }
    closeList();
    paragraph.push(line);
  }
  flushParagraph();
  closeList();
  return html.join("");
}

// --- El libro busca hoja por hoja ------------------------------------------

const seek = { active: false, startedAt: 0, page: 1, flips: 0, flipTimer: 0, fallTimer: 0 };

function openSeek() {
  seek.active = true;
  seek.startedAt = performance.now();
  seek.page = 1;
  seek.flips = 0;
  seekOverlay.hidden = false;
  seekOverlay.classList.remove("is-closing");
  void seekOverlay.offsetWidth;
  seekOverlay.classList.add("is-open");
  document.body.classList.add("is-seeking");
  updateSeekProgress();
  seekStack.style.setProperty("--stack", "1");
  randomizePage(seekBodyLeft);
  randomizePage(seekBodyRight);
  if (reducedMotion) return;
  scheduleFlip(520);
  seek.fallTimer = window.setInterval(spawnFallingPage, 780);
  for (let i = 0; i < 3; i += 1) window.setTimeout(spawnFallingPage, 320 + i * 260);
}

function updateSeekProgress() {
  seekProgress.textContent = `HOJA ${seek.page} / ${TOTAL_FOLIOS}`;
}

function scheduleFlip(delay) {
  seek.flipTimer = window.setTimeout(runFlip, delay);
}

function runFlip() {
  if (!seek.active) return;
  seek.page = Math.min(TOTAL_FOLIOS, seek.page + 1 + Math.floor(Math.random() * 3));
  seek.flips += 1;
  updateSeekProgress();
  seekStack.style.setProperty("--stack", String(Math.max(0.08, 1 - seek.flips * 0.055)));
  seekBodyFlip.innerHTML = seekBodyRight.innerHTML;
  const duration = 105 + Math.random() * 85;
  seekFlip.style.setProperty("--flip-duration", `${Math.round(duration)}ms`);
  seekFlip.classList.remove("is-flipping");
  void seekFlip.offsetWidth;
  seekFlip.classList.add("is-flipping");
  seek.flipTimer = window.setTimeout(() => {
    seekFlip.classList.remove("is-flipping");
    seekBodyLeft.innerHTML = seekBodyFlip.innerHTML;
    randomizePage(seekBodyRight);
    scheduleFlip(15 + Math.random() * 55);
  }, duration);
}

function spawnFallingPage() {
  if (!seek.active || seekFall.childElementCount >= 7) return;
  const page = document.createElement("span");
  page.className = "falling-page";
  page.style.left = `${8 + Math.random() * 82}%`;
  page.style.setProperty("--fall-x", `${Math.round((Math.random() * 2 - 1) * 130)}px`);
  page.style.setProperty("--fall-spin", `${Math.round((Math.random() * 2 - 1) * 320)}deg`);
  page.style.setProperty("--fall-duration", `${(2.3 + Math.random() * 1.9).toFixed(2)}s`);
  page.style.setProperty("--fall-delay", `${(Math.random() * 0.35).toFixed(2)}s`);
  page.addEventListener("animationend", () => page.remove(), { once: true });
  seekFall.append(page);
}

async function closeSeek() {
  clearTimeout(seek.flipTimer);
  clearInterval(seek.fallTimer);
  seekFlip.classList.remove("is-flipping");
  const elapsed = performance.now() - seek.startedAt;
  const remaining = 1500 - elapsed;
  if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining));
  seekOverlay.classList.remove("is-open");
  seekOverlay.classList.add("is-closing");
  await new Promise((resolve) => setTimeout(resolve, 480));
  seekOverlay.classList.remove("is-closing");
  seekOverlay.hidden = true;
  document.body.classList.remove("is-seeking");
  seekFall.replaceChildren();
  seek.active = false;
}

// A layered, slow-moving blood-red sea drawn without images or dependencies.
const canvas = document.querySelector("#bloodSea");
const context = canvas.getContext("2d", { alpha: true });
let seaWidth = 0;
let seaHeight = 0;
let seaFrame = 0;

function resizeSea() {
  const scale = Math.min(window.devicePixelRatio || 1, 1.5);
  seaWidth = window.innerWidth;
  seaHeight = Math.max(260, window.innerHeight * 0.44);
  canvas.width = Math.round(seaWidth * scale);
  canvas.height = Math.round(seaHeight * scale);
  context.setTransform(scale, 0, 0, scale, 0, 0);
}

function drawWave(time, baseline, amplitude, frequency, speed, color, phase) {
  context.beginPath();
  context.moveTo(0, seaHeight);
  const step = Math.max(5, seaWidth / 180);
  for (let x = 0; x <= seaWidth + step; x += step) {
    const ripple = Math.sin(x * frequency + time * speed + phase) * amplitude;
    const swell = Math.sin(x * frequency * 0.37 - time * speed * 0.58 + phase) * amplitude * 0.58;
    const y = baseline + ripple + swell;
    context.lineTo(x, y);
  }
  context.lineTo(seaWidth, seaHeight);
  context.closePath();
  context.fillStyle = color;
  context.fill();
}

function paintSea(time = 0) {
  context.clearRect(0, 0, seaWidth, seaHeight);
  const t = reducedMotion ? 0 : time * 0.00022;
  drawWave(t, seaHeight * 0.47, 17, 0.009, 1.1, "rgba(81, 13, 20, .40)", .2);
  drawWave(t, seaHeight * 0.57, 22, 0.012, .82, "rgba(105, 18, 26, .31)", 2.6);
  drawWave(t, seaHeight * 0.68, 16, 0.017, .66, "rgba(134, 25, 31, .29)", 4.8);
  drawWave(t, seaHeight * 0.79, 12, 0.022, .48, "rgba(77, 13, 19, .55)", 1.7);
  if (!reducedMotion) seaFrame = requestAnimationFrame(paintSea);
}

resizeSea();
paintSea();
window.addEventListener("resize", () => {
  cancelAnimationFrame(seaFrame);
  resizeSea();
  paintSea();
}, { passive: true });

// The tome follows the cursor with a subtle parallax tilt on larger screens.
const book = document.querySelector(".book-stage");
if (!reducedMotion && window.matchMedia("(pointer: fine)").matches) {
  document.addEventListener("pointermove", (event) => {
    const x = (event.clientX / window.innerWidth - 0.5) * 5;
    const y = (event.clientY / window.innerHeight - 0.5) * -4;
    book.style.transform = `rotateY(${x}deg) rotateX(${y}deg)`;
  }, { passive: true });
  document.addEventListener("pointerleave", () => { book.style.transform = ""; });
}
