import { renderCard } from "./cards.js";

const $ = (sel, root = document) => root.querySelector(sel);

/* ---------- theme ---------- */
// index.html sets data-theme inline before paint; this wires the toggle.
$(".theme-toggle").addEventListener("click", () => {
  const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = next;
  localStorage.setItem("theme", next);
});

/* ---------- elements ---------- */
const main = $("#main");
const rail = $("#rail");
const status = $("#status");
const forms = document.querySelectorAll(".searchwrap form");

let stream = null;

/* ---------- routing ---------- */
function go(q, push = true) {
  q = q.trim();
  if (!q) return;
  if (push) history.pushState({ q }, "", "/search?q=" + encodeURIComponent(q));
  document.title = q + " - shirabe";
  forms.forEach((f) => (f.q.value = q));
  showResults();
  runQuery(q);
}

window.addEventListener("popstate", () => route(false));

function route(push = true) {
  const q = new URL(location.href).searchParams.get("q") || "";
  if (location.pathname === "/search" && q) {
    go(q, push);
  } else if (location.pathname === "/dev/cards") {
    showResults();
    devCards();
  } else {
    showHome();
  }
}

function showHome() {
  document.body.classList.remove("mode-results");
  document.title = "shirabe";
  $("#q-home").focus();
}

function showResults() {
  document.body.classList.add("mode-results");
}

/* ---------- query streaming ---------- */
function runQuery(q) {
  if (stream) stream.close();
  main.innerHTML = "";
  rail.innerHTML = "";
  status.innerHTML = "";
  const skeletons = [];
  for (let i = 0; i < 3; i++) {
    const sk = document.createElement("div");
    sk.className = "card skeleton";
    sk.innerHTML = '<div class="bar w40"></div><div class="bar"></div><div class="bar w60"></div>';
    main.appendChild(sk);
    skeletons.push(sk);
  }

  let imgrid = null;
  let count = 0;

  stream = new EventSource("/api/query?q=" + encodeURIComponent(q));

  stream.addEventListener("cards", (ev) => {
    const cards = JSON.parse(ev.data).cards || [];
    if (cards.length && count === 0) skeletons.forEach((s) => s.remove());
    for (const card of cards) {
      count++;
      placeCard(card, () => {
        if (!imgrid) {
          imgrid = document.createElement("div");
          imgrid.className = "imgrid";
          main.appendChild(imgrid);
        }
        return imgrid;
      });
    }
  });

  stream.addEventListener("done", (ev) => {
    stream.close();
    stream = null;
    skeletons.forEach((s) => s.remove());
    if (count === 0) {
      main.innerHTML = `<div class="empty-state"><div class="kanji">無</div><p>Nothing found for "${escapeText(q)}".</p><p>Try a URL, a plain search, or a bang like !yt or !wiki.</p></div>`;
    }
    let done = {};
    try { done = JSON.parse(ev.data) || {}; } catch { /* older server */ }
    let footer = "";
    if (done.errors?.length) {
      footer += `<div class="quiet-errors">${done.errors.map((e) =>
        `<div class="row"><b>${escapeText(e.source)}</b>${escapeText(e.message)}</div>`).join("")}</div>`;
    }
    if (done.timings?.length) {
      footer += `<div class="timing-chips">${done.timings.map((x) =>
        `<span>${escapeText(x.source)} ${x.ms}ms</span>`).join("")}</div>`;
    }
    status.innerHTML = footer;
  });

  // EventSource transport failure (server down, network drop).
  stream.onerror = () => {
    if (stream && stream.readyState === EventSource.CLOSED) {
      skeletons.forEach((s) => s.remove());
      if (count === 0) {
        main.innerHTML = '<div class="empty-state"><div class="kanji">断</div><p>Lost the connection to the server.</p></div>';
      }
      stream = null;
    }
  };
}

function placeCard(card, getImgrid) {
  const node = renderCard(card);
  // The first entity becomes the knowledge panel when the rail is visible.
  if (card.kind === "entity" && rail.offsetParent !== null && !rail.hasChildNodes()) {
    rail.appendChild(node);
    return;
  }
  if (card.kind === "image") {
    getImgrid().appendChild(node);
    return;
  }
  // Answer-grade cards (intent hits) go to the top of the column.
  if (card.score >= 1 && main.firstChild) {
    main.insertBefore(node, main.firstChild);
  } else {
    main.appendChild(node);
  }
}

function escapeText(s) {
  const d = document.createElement("div");
  d.textContent = String(s ?? "");
  return d.innerHTML;
}

/* ---------- dev fixtures ---------- */
async function devCards() {
  document.title = "dev cards - shirabe";
  main.innerHTML = "";
  rail.innerHTML = "";
  status.innerHTML = "";
  const res = await fetch("/api/dev/cards");
  if (!res.ok) {
    main.innerHTML = '<div class="empty-state"><p>Dev cards need <code>shirabe serve --dev</code>.</p></div>';
    return;
  }
  const cards = (await res.json()).cards || [];
  let imgrid = null;
  for (const card of cards) {
    placeCard(card, () => {
      if (!imgrid) {
        imgrid = document.createElement("div");
        imgrid.className = "imgrid";
        main.appendChild(imgrid);
      }
      return imgrid;
    });
  }
}

/* ---------- suggest ---------- */
function wireSuggest(form) {
  const input = form.q;
  const box = form.closest(".searchwrap").querySelector(".suggest");
  const list = box.querySelector("ul");
  let items = [], sel = -1, timer = null, ctl = null;

  const close = () => { box.classList.remove("open"); items = []; sel = -1; };

  const render = () => {
    list.innerHTML = items.map((s, i) =>
      `<li class="${i === sel ? "sel" : ""}" data-i="${i}">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.5-4.5"/></svg>
        ${escapeText(s)}</li>`).join("");
    box.classList.toggle("open", items.length > 0);
  };

  input.addEventListener("input", () => {
    clearTimeout(timer);
    const v = input.value.trim();
    if (v.length < 2 || v.includes("://") || v.startsWith("!")) { close(); return; }
    timer = setTimeout(async () => {
      if (ctl) ctl.abort();
      ctl = new AbortController();
      try {
        const res = await fetch("/api/suggest?q=" + encodeURIComponent(v), { signal: ctl.signal });
        items = (await res.json()).slice(0, 8);
        sel = -1;
        render();
      } catch { /* aborted or offline */ }
    }, 160);
  });

  input.addEventListener("keydown", (e) => {
    if (!box.classList.contains("open")) return;
    if (e.key === "ArrowDown") { e.preventDefault(); sel = (sel + 1) % items.length; render(); }
    else if (e.key === "ArrowUp") { e.preventDefault(); sel = (sel - 1 + items.length) % items.length; render(); }
    else if (e.key === "Enter" && sel >= 0) { input.value = items[sel]; close(); }
    else if (e.key === "Escape") close();
  });

  box.addEventListener("pointerdown", (e) => {
    const item = e.target.closest("li");
    if (!item) return;
    e.preventDefault();
    input.value = items[Number(item.dataset.i)];
    close();
    form.requestSubmit();
  });

  input.addEventListener("blur", () => setTimeout(close, 120));
}

/* ---------- wiring ---------- */
forms.forEach((form) => {
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    go(form.q.value);
  });
  wireSuggest(form);
});

$(".mark").addEventListener("click", (e) => {
  e.preventDefault();
  history.pushState({}, "", "/");
  route(false);
});

document.addEventListener("keydown", (e) => {
  if (e.key === "/" && !/^(INPUT|TEXTAREA)$/.test(document.activeElement.tagName)) {
    e.preventDefault();
    const input = document.body.classList.contains("mode-results") ? $("#q-top") : $("#q-home");
    input.focus();
    input.select();
  }
});

route(false);
