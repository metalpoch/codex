const form = document.querySelector("#searchForm");
const input = document.querySelector("#question");
const button = form.querySelector("button[type=submit]");
const errorMessage = document.querySelector("#errorMessage");
const scriptFeed = document.querySelector("#scriptFeed");
const scriptEmpty = document.querySelector("#scriptEmpty");
const scriptCount = document.querySelector("#scriptCount");
const seekOverlay = document.querySelector("#seekOverlay");
const seekStack = document.querySelector("#seekStack");
const seekQuip = document.querySelector("#seekQuip");
const seekFall = document.querySelector("#seekFall");
const seekSheets = document.querySelector("#seekSheets");
const seekTurnShadow = document.querySelector("#seekTurnShadow");
const seekBodyLeft = document.querySelector("#seekBodyLeft");
const seekBodyRight = document.querySelector("#seekBodyRight");

const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const SVG_NS = "http://www.w3.org/2000/svg";

const INK_GLYPHS = ["ᚠ", "ᚢ", "ᚦ", "ᚨ", "ᚱ", "ᚲ", "ᚷ", "ᚹ", "ᚺ", "ᚾ", "ᛁ", "ᛃ", "ᛇ", "ᛈ", "ᛉ", "ᛊ", "ᛏ", "ᛒ", "ᛖ", "ᛗ", "ᛚ", "ᛞ", "ᛟ", "✶", "☽", "✥", "⚚", "☤"];

const DOODLES = {
  seal: '<circle cx="20" cy="20" r="17.2"/><path d="M20 5.5 26.9 25.8 8.6 13.2h22.8L13.1 25.8Z"/>',
  cross: '<path d="M20 4v32M13 26h14"/>',
  eye: '<path d="M4 20c5-7 10-10 16-10s11 3 16 10c-5 7-10 10-16 10S9 27 4 20Z"/><circle cx="20" cy="20" r="5"/><circle class="pupil" cx="20" cy="20" r="1.7"/>',
  bat: '<path d="M20 15c-2-6-6-8-10-6 1 3 0 6-4 8 3 1 4 3 4 6 3-3 7-4 10-2 3-2 7-1 10 2 0-3 1-5 4-6-4-2-5-5-4-8-4-2-8 0-10 6Z"/>',
  skull: '<path d="M12 17a8 9 0 1 1 16 0c0 4-1 6-3 8v3H15v-3c-2-2-3-4-3-8Z"/><circle cx="16" cy="19" r="2.2"/><circle cx="24" cy="19" r="2.2"/><path d="M18 28v3M22 28v3"/>',
  crown: '<path d="M8 28 12 12l8 8 8-12 4 16Z"/><path d="M8 28h24"/>',
  spiral: '<path d="M20 20c0-2 2-3 4-2s2 5-1 7-8-1-8-6 5-9 11-8 12 5 12 11-8 11-14 9"/>',
  dagger: '<path d="M20 4 23 12v14l-3 6-3-6V12Z"/><path d="M12 14h16M20 32v4"/>',
  moon: '<path d="M26 6a14 14 0 1 0 0 28 11 11 0 0 1 0-28Z"/>',
  star: '<path d="m20 5 4 11h12l-10 7 4 12-10-7-10 7 4-12-10-7h12Z"/>',
  flame: '<path d="M20 34c-6 0-10-4-10-9 0-7 8-9 7-18 6 3 13 10 13 18 0 5-4 9-10 9Z"/>',
  arrow: '<path d="M6 28 30 12M30 12l-8 1M30 12l-1 8"/>',
  horns: '<path d="M8 30c0-10 4-18 8-22M32 30c0-10-4-18-8-22M12 26c5-4 11-4 16 0"/>',
  chalice: '<path d="M12 8h16c0 8-3 12-8 13-5-1-8-5-8-13ZM20 21v9M14 32h12"/>',
};

const SEEK_QUIPS = [
  "Buscando la página que Kael se quería fumar",
  "Quitando la grasa de una empanada de mechada",
  "Buscando la receta del porro de Kael",
  "Preguntando a la Camarilla quién dejó migas en el tomo",
  "Separando las hojas que huelen a mechada",
  "Consultando el índice de recetas de la noche",
];

function randomBetween(min, max) {
  return min + Math.random() * (max - min);
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// --- Manuscrito: trazos temblorosos en lugar de renglones rectos ------------

function wordPath(x0, width) {
  const bumps = Math.max(2, Math.round(width / randomBetween(3.2, 5.8)));
  const seg = width / bumps;
  let d = `M ${x0.toFixed(1)} 5`;
  for (let i = 0; i < bumps; i += 1) {
    const amp = randomBetween(1.1, 2.5) * (i % 2 === 0 ? -1 : 1);
    d += ` q ${(seg / 2).toFixed(1)} ${amp.toFixed(1)} ${seg.toFixed(1)} 0`;
  }
  return d;
}

function scribbleLine({ width = 90, thick = false, red = false, faded = false, strike = false }) {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("viewBox", "0 0 100 10");
  svg.setAttribute("preserveAspectRatio", "none");
  svg.classList.add("seek-scribble");
  svg.style.width = `${width.toFixed(1)}%`;
  const alpha = faded ? .28 : red ? .5 : .42;
  const color = red ? `rgba(142, 32, 37, ${alpha})` : `rgba(74, 56, 42, ${alpha})`;
  const addPath = (d, strokeWidth) => {
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", d);
    path.setAttribute("fill", "none");
    path.setAttribute("stroke", color);
    path.setAttribute("stroke-width", String(strokeWidth));
    path.setAttribute("vector-effect", "non-scaling-stroke");
    svg.append(path);
  };
  let x = randomBetween(0, 3);
  let d = "";
  const words = 2 + Math.floor(Math.random() * 4);
  for (let i = 0; i < words && x < 96; i += 1) {
    const wordWidth = randomBetween(9, 24);
    if (x + wordWidth > 98) break;
    d += ` ${wordPath(x, wordWidth)}`;
    x += wordWidth + randomBetween(2.2, 5);
  }
  addPath(d.trim(), thick ? 1.9 : randomBetween(1, 1.4));
  if (strike) {
    const y = randomBetween(4.2, 6);
    addPath(`M 0 ${y.toFixed(1)} q 25 ${(randomBetween(-1.4, 1.4)).toFixed(1)} 50 0 t 50 ${randomBetween(-.8, .8).toFixed(1)}`, 1.1);
    if (Math.random() < 0.35) addPath(`M 2 ${(y - 1).toFixed(1)} q 30 1 60 -.5 t 36 0`, .9);
  }
  return svg;
}

function doodleEl() {
  const span = document.createElement("span");
  span.className = "seek-doodle";
  if (Math.random() < 0.22) span.classList.add("is-red");
  span.style.width = `${randomBetween(26, 46).toFixed(0)}px`;
  span.style.height = span.style.width;
  span.style.transform = `rotate(${randomBetween(-24, 24).toFixed(1)}deg)`;
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("viewBox", "0 0 40 40");
  svg.innerHTML = pick(Object.values(DOODLES));
  span.append(svg);
  return span;
}

function stainEl() {
  const span = document.createElement("span");
  span.className = "seek-stain";
  const size = randomBetween(22, 58);
  const red = Math.random() < 0.45;
  span.style.width = `${size.toFixed(0)}px`;
  span.style.height = `${(size * randomBetween(.7, 1.1)).toFixed(0)}px`;
  span.style.background = red
    ? `radial-gradient(circle, rgba(112, 24, 28, ${randomBetween(.14, .3).toFixed(2)}), transparent 68%)`
    : `radial-gradient(circle, rgba(96, 72, 44, ${randomBetween(.12, .24).toFixed(2)}), transparent 68%)`;
  return span;
}

function fillColumn(col) {
  let top = randomBetween(1, 4);
  while (top < 90) {
    const lines = 2 + Math.floor(Math.random() * 4);
    for (let i = 0; i < lines && top < 90; i += 1) {
      const last = i === lines - 1;
      const line = scribbleLine({
        width: last ? randomBetween(40, 82) : randomBetween(76, 100),
        thick: Math.random() < 0.07,
        red: Math.random() < 0.08,
        faded: Math.random() < 0.12,
      });
      line.style.top = `${top.toFixed(1)}%`;
      col.append(line);
      top += randomBetween(5, 6.6);
      if (Math.random() < 0.1) {
        const strike = scribbleLine({ width: randomBetween(55, 100), strike: true, faded: true });
        strike.style.top = `${(top - 5.4).toFixed(1)}%`;
        strike.style.left = `${randomBetween(0, 8).toFixed(1)}%`;
        col.append(strike);
      }
    }
    top += randomBetween(2.2, 5.4);
    if (Math.random() < 0.15 && top < 68) {
      const doodle = doodleEl();
      doodle.style.top = `${top.toFixed(1)}%`;
      doodle.style.left = `${randomBetween(4, 48).toFixed(1)}%`;
      col.append(doodle);
      top += randomBetween(13, 18);
    }
    if (Math.random() < 0.1 && top < 80) {
      const stain = stainEl();
      stain.style.top = `${randomBetween(10, 80).toFixed(1)}%`;
      stain.style.left = `${randomBetween(6, 70).toFixed(1)}%`;
      col.append(stain);
    }
  }
}

function randomizePage(body) {
  body.replaceChildren();
  const addPos = (className, style) => {
    const el = document.createElement("span");
    el.className = className;
    Object.assign(el.style, style);
    body.append(el);
    return el;
  };
  addPos("seek-rubric", { top: `${randomBetween(8, 13).toFixed(1)}%`, left: "11%", width: `${randomBetween(24, 46).toFixed(1)}%` });
  if (Math.random() < 0.55) {
    const subtitle = scribbleLine({ width: randomBetween(28, 52), thick: true, red: Math.random() < 0.6 });
    subtitle.style.top = `${randomBetween(14.5, 17).toFixed(1)}%`;
    subtitle.style.left = "11%";
    body.append(subtitle);
  }
  if (Math.random() < 0.35) {
    const left = document.createElement("div");
    left.className = "seek-col";
    left.style.left = "9%";
    left.style.width = "39%";
    const right = document.createElement("div");
    right.className = "seek-col";
    right.style.left = "53%";
    right.style.width = "38%";
    body.append(left, right);
    fillColumn(left);
    fillColumn(right);
  } else {
    const col = document.createElement("div");
    col.className = "seek-col";
    col.style.left = "11%";
    col.style.width = "78%";
    body.append(col);
    fillColumn(col);
  }
  if (Math.random() < 0.55) {
    addPos("seek-ink", {
      top: `${randomBetween(18, 40).toFixed(1)}%`,
      left: `${randomBetween(2, 6).toFixed(1)}%`,
      fontSize: `${randomBetween(10, 14).toFixed(1)}px`,
    }).textContent = pick(INK_GLYPHS);
  }
  if (Math.random() < 0.3) {
    addPos("seek-ink", {
      top: `${randomBetween(48, 74).toFixed(1)}%`,
      right: `${randomBetween(2, 6).toFixed(1)}%`,
      fontSize: `${randomBetween(9, 12).toFixed(1)}px`,
    }).textContent = pick(INK_GLYPHS);
  }
  if (Math.random() < 0.35) {
    const doodle = doodleEl();
    doodle.style.top = `${randomBetween(55, 78).toFixed(1)}%`;
    doodle.style.right = `${randomBetween(6, 14).toFixed(1)}%`;
    body.append(doodle);
  }
  if (Math.random() < 0.18) {
    const stain = stainEl();
    stain.style.top = `${randomBetween(12, 70).toFixed(1)}%`;
    stain.style.left = `${randomBetween(8, 60).toFixed(1)}%`;
    body.append(stain);
  }
}

// --- Scriptorium: la respuesta se escribe -----------------------------------

function buildEntry(question) {
  scriptEmpty.hidden = true;
  const entry = document.createElement("article");
  entry.className = "entry is-pending";
  entry.innerHTML = `
    <header class="entry-head">
      <span class="entry-mark" aria-hidden="true">✦</span>
      <span class="entry-question"></span>
    </header>
    <div class="entry-copy">El erudito hojea el archivo…</div>
    <button class="entry-skip" type="button" hidden>Mostrar completa</button>
    <details class="sources" hidden>
      <summary><span>Fragmentos consultados</span><span class="source-count"></span></summary>
      <div class="source-list"></div>
    </details>`;
  entry.querySelector(".entry-question").textContent = question;
  scriptFeed.prepend(entry);
  const total = scriptFeed.childElementCount;
  scriptCount.textContent = total ? `· ${total} ${total === 1 ? "CONSULTA" : "CONSULTAS"}` : "";
  return entry;
}

function fillSources(entry, sources) {
  const details = entry.querySelector(".sources");
  const list = entry.querySelector(".source-list");
  const count = entry.querySelector(".source-count");
  list.replaceChildren();
  for (const source of sources) {
    const item = document.createElement("p");
    item.className = "source-excerpt";
    item.append(document.createTextNode(source.excerpt));
    const score = document.createElement("span");
    score.className = "source-score";
    score.textContent = `RELEVANCIA · ${Math.round(source.similarity * 100)}%`;
    item.append(score);
    list.append(item);
  }
  count.textContent = sources.length ? `· ${sources.length}` : "";
  details.hidden = sources.length === 0;
}

function streamEntry(entry, fullText) {
  const copy = entry.querySelector(".entry-copy");
  const skip = entry.querySelector(".entry-skip");
  entry.classList.remove("is-pending");
  if (reducedMotion) {
    copy.innerHTML = renderMarkdown(fullText);
    return Promise.resolve();
  }
  skip.hidden = false;
  return new Promise((resolve) => {
    const parts = String(fullText).split(/(\s+)/);
    const batch = fullText.length > 1100 ? 5 : fullText.length > 500 ? 3 : 2;
    let index = 0;
    let shown = "";
    const finish = () => {
      copy.innerHTML = renderMarkdown(fullText);
      skip.hidden = true;
      skip.removeEventListener("click", onStop);
      resolve();
    };
    const onStop = () => { finish(); };
    skip.addEventListener("click", onStop);
    const tick = () => {
      for (let i = 0; i < batch && index < parts.length; i += 1, index += 1) shown += parts[index];
      copy.innerHTML = renderMarkdown(shown) + '<span class="entry-caret" aria-hidden="true">✒</span>';
      if (index >= parts.length) return finish();
      window.setTimeout(tick, 55);
    };
    window.setTimeout(tick, 140);
  });
}

async function streamAnswer(entry, response) {
  const copy = entry.querySelector(".entry-copy");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const handleEvent = async (event) => {
    if (event.type === "sources") {
      fillSources(entry, event.sources || []);
    } else if (event.type === "delta" && event.text) {
      answer += event.text;
      entry.classList.remove("is-pending");
      if (seek.active) await closeSeek();
      copy.innerHTML =
        renderMarkdown(answer) +
        (reducedMotion ? "" : '<span class="entry-caret" aria-hidden="true">✒</span>');
    } else if (event.type === "error") {
      throw new Error(event.message || "La respuesta del erudito falló.");
    }
  };
  let answer = "";
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let boundary;
    while ((boundary = buffer.indexOf("\n\n")) >= 0) {
      const raw = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      for (const line of raw.split("\n")) {
        if (!line.startsWith("data:")) continue;
        const payload = line.slice(5).trim();
        if (!payload) continue;
        let event;
        try {
          event = JSON.parse(payload);
        } catch {
          continue;
        }
        await handleEvent(event);
      }
    }
  }
  if (!answer) throw new Error("El erudito no devolvió ninguna respuesta.");
  copy.innerHTML = renderMarkdown(answer);
}

async function runQuery(question) {
  const entry = buildEntry(question);
  const copy = entry.querySelector(".entry-copy");
  button.disabled = true;
  errorMessage.hidden = true;
  openSeek();
  try {
    const response = await fetch("/api/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ question }),
    });
    if (!response.ok) {
      const payload = await response.json().catch(() => ({}));
      throw new Error(payload.error || "No se pudo consultar el archivo.");
    }
    const contentType = response.headers.get("content-type") || "";
    if (contentType.includes("text/event-stream")) {
      await streamAnswer(entry, response);
    } else {
      const payload = await response.json();
      await closeSeek();
      await streamEntry(entry, payload.answer);
      fillSources(entry, payload.sources || []);
    }
  } catch (error) {
    await closeSeek();
    entry.classList.remove("is-pending");
    entry.classList.add("is-error");
    copy.textContent = error.message;
    errorMessage.textContent = error.message;
    errorMessage.hidden = false;
  } finally {
    button.disabled = false;
  }
}

form.addEventListener("submit", (event) => {
  event.preventDefault();
  const question = input.value.trim();
  if (!question) return;
  runQuery(question);
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

// --- El libro busca, hoja tras hoja -----------------------------------------

const SHEET_COUNT = 5;
const seek = { active: false, startedAt: 0, turns: 0, cursor: 0, turnTimer: 0, fallTimer: 0, quipTimer: 0, quipIndex: 0 };
const sheets = [];

function ensureSheets() {
  if (sheets.length) return;
  for (let i = 0; i < SHEET_COUNT; i += 1) {
    const sheet = document.createElement("div");
    sheet.className = "seek-sheet";
    sheet.style.setProperty("--fan", String(i));
    const turn = document.createElement("div");
    turn.className = "seek-sheet-turn";
    const body = document.createElement("div");
    body.className = "seek-page-body";
    const cast = document.createElement("div");
    cast.className = "seek-sheet-cast";
    turn.append(body, cast);
    sheet.append(turn);
    seekSheets.append(sheet);
    const entry = { turn, body };
    turn.addEventListener("animationend", () => {
      turn.classList.remove("is-turning");
      if (seek.active) seekBodyLeft.innerHTML = body.innerHTML;
    });
    sheets.push(entry);
  }
}

function openSeek() {
  seek.active = true;
  seek.startedAt = performance.now();
  seek.turns = 0;
  seekOverlay.hidden = false;
  seekOverlay.classList.remove("is-closing");
  void seekOverlay.offsetWidth;
  seekOverlay.classList.add("is-open");
  document.body.classList.add("is-seeking");
  seekStack.style.setProperty("--stack", "1");
  randomizePage(seekBodyLeft);
  randomizePage(seekBodyRight);
  seek.quipIndex = Math.floor(Math.random() * SEEK_QUIPS.length);
  seekQuip.textContent = SEEK_QUIPS[seek.quipIndex];
  seekQuip.classList.remove("is-visible");
  void seekQuip.offsetWidth;
  seekQuip.classList.add("is-visible");
  seek.quipTimer = window.setTimeout(rotateQuip, 2800);
  if (reducedMotion) return;
  ensureSheets();
  seek.turnTimer = window.setTimeout(runTurn, 480);
  seek.fallTimer = window.setInterval(spawnFallingPage, 820);
  for (let i = 0; i < 3; i += 1) window.setTimeout(spawnFallingPage, 340 + i * 280);
}

function rotateQuip() {
  if (!seek.active) return;
  seek.quipIndex = (seek.quipIndex + 1) % SEEK_QUIPS.length;
  seekQuip.classList.remove("is-visible");
  seek.quipTimer = window.setTimeout(() => {
    if (!seek.active) return;
    seekQuip.textContent = SEEK_QUIPS[seek.quipIndex];
    seekQuip.classList.add("is-visible");
    seek.quipTimer = window.setTimeout(rotateQuip, 2800);
  }, 320);
}

function runTurn() {
  if (!seek.active) return;
  const sheet = sheets[seek.cursor % SHEET_COUNT];
  seek.cursor += 1;
  seek.turns += 1;
  seekStack.style.setProperty("--stack", String(Math.max(0.06, 1 - seek.turns * 0.05)));
  sheet.body.innerHTML = seekBodyRight.innerHTML;
  const durationMs = `${Math.round(250 + Math.random() * 120)}ms`;
  sheet.turn.style.setProperty("--turn-duration", durationMs);
  sheet.turn.classList.remove("is-turning");
  void sheet.turn.offsetWidth;
  sheet.turn.classList.add("is-turning");
  seekTurnShadow.classList.remove("is-turning");
  seekTurnShadow.style.setProperty("--turn-duration", durationMs);
  void seekTurnShadow.offsetWidth;
  seekTurnShadow.classList.add("is-turning");
  randomizePage(seekBodyRight);
  seek.turnTimer = window.setTimeout(runTurn, 100 + Math.random() * 70);
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
  clearTimeout(seek.turnTimer);
  clearInterval(seek.fallTimer);
  clearTimeout(seek.quipTimer);
  seekQuip.classList.remove("is-visible");
  for (const sheet of sheets) sheet.turn.classList.remove("is-turning");
  seekTurnShadow.classList.remove("is-turning");
  const elapsed = performance.now() - seek.startedAt;
  const remaining = 1300 - elapsed;
  if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining));
  seekOverlay.classList.remove("is-open");
  seekOverlay.classList.add("is-closing");
  await new Promise((resolve) => setTimeout(resolve, 480));
  seekOverlay.classList.remove("is-closing");
  seekOverlay.hidden = true;
  document.body.classList.remove("is-seeking");
  seekFall.replaceChildren();
  seekQuip.textContent = "";
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
