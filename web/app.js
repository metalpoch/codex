const form = document.querySelector("#searchForm");
const input = document.querySelector("#question");
const button = form.querySelector("button[type=submit]");
const answerArea = document.querySelector("#answerArea");
const answerText = document.querySelector("#answerText");
const loadingState = document.querySelector("#loadingState");
const errorMessage = document.querySelector("#errorMessage");
const sourcesDetails = document.querySelector("#sourcesDetails");
const sourceList = document.querySelector("#sourceList");
const sourceCount = document.querySelector("#sourceCount");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const question = input.value.trim();
  if (!question) return;

  button.disabled = true;
  answerArea.hidden = true;
  errorMessage.hidden = true;
  loadingState.hidden = false;
  sourcesDetails.open = false;

  try {
    const response = await fetch("/api/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ question }),
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || "No se pudo consultar el archivo.");

    answerText.textContent = payload.answer;
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
    loadingState.hidden = true;
    button.disabled = false;
  }
});

document.querySelectorAll("[data-question]").forEach((suggestion) => {
  suggestion.addEventListener("click", () => {
    input.value = suggestion.dataset.question;
    form.requestSubmit();
  });
});

// A layered, slow-moving blood-red sea drawn without images or dependencies.
const canvas = document.querySelector("#bloodSea");
const context = canvas.getContext("2d", { alpha: true });
const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
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
