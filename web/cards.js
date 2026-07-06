// Card renderers. One function per kind; unknown kinds fall back to the web
// renderer so a newer server never breaks an older UI.

const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

const img = (u) => u ? `/img?u=${encodeURIComponent(u)}` : "";

const el = (html) => {
  const t = document.createElement("template");
  t.innerHTML = html.trim();
  return t.content.firstElementChild;
};

const num = (n) => {
  n = Number(n) || 0;
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
  return String(n);
};

const stars = (r) => {
  const full = Math.round(Number(r) || 0);
  return "★".repeat(Math.min(full, 5)) + "☆".repeat(Math.max(0, 5 - full));
};

function hostOf(u) {
  try { return new URL(u).hostname.replace(/^www\./, ""); } catch { return ""; }
}

const siteRow = (card) => {
  const host = hostOf(card.url);
  if (!host) return "";
  return `<div class="site"><span class="dot">${esc(host[0] || "?")}</span>${esc(host)}</div>`;
};

const title = (card) =>
  card.url
    ? `<h3><a href="${esc(card.url)}" rel="noopener" target="_blank">${esc(card.title)}</a></h3>`
    : `<h3>${esc(card.title)}</h3>`;

const chip = (card) => `<span class="source-chip">${esc(card.source)}</span>`;
const snippet = (card) => card.snippet ? `<p class="snippet">${esc(card.snippet)}</p>` : "";

// Weather glyphs, drawn inline so the binary needs no icon font.
const ICONS = {
  sun: '<circle cx="12" cy="12" r="4.5"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2"/>',
  "sun-cloud": '<circle cx="8.5" cy="8" r="3.4"/><path d="M8.5 1.8v2M2.3 8h2M3.6 3.6l1.5 1.5"/><path d="M8 20h9a3.5 3.5 0 0 0 .5-6.97A5 5 0 0 0 8 14a3 3 0 0 0 0 6z"/>',
  cloud: '<path d="M6.5 19h11a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 12.1 3.5 3.5 0 0 0 6.5 19z"/>',
  fog: '<path d="M4 10h16M2 14h20M5 18h14"/>',
  drizzle: '<path d="M6.5 14h11a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 7.1 3.5 3.5 0 0 0 6.5 14z"/><path d="M8 17v2M12 17v3M16 17v2"/>',
  rain: '<path d="M6.5 13h11a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 6.1 3.5 3.5 0 0 0 6.5 13z"/><path d="M8 16l-1 4M12.5 16l-1 4M17 16l-1 4"/>',
  snow: '<path d="M6.5 13h11a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 6.1 3.5 3.5 0 0 0 6.5 13z"/><path d="M8 17.5h.01M12 19.5h.01M16 17.5h.01M10 21h.01M14 21.5h.01"/>',
  storm: '<path d="M6.5 13h11a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 6.1 3.5 3.5 0 0 0 6.5 13z"/><path d="M12.5 14l-2.5 4h3l-2 4.5"/>',
};

const icon = (name, size = 24) =>
  `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">${ICONS[name] || ICONS.cloud}</svg>`;

const PLAY = '<svg width="34" height="34" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5.5v13l11-6.5z"/></svg>';

/* ---------- kind renderers ---------- */

function web(card) {
  const b = card.body || {};
  return el(`<article class="card">${chip(card)}
    <div class="site"><span class="dot">${esc((b.site || hostOf(card.url))[0] || "?")}</span>${esc(b.display_url || b.site || hostOf(card.url))}</div>
    ${title(card)}${snippet(card)}
  </article>`);
}

function video(card) {
  const b = card.body || {};
  const meta = [b.channel, b.views ? num(b.views) + " views" : "", b.published, ]
    .filter(Boolean).map(esc).join('</span><span class="sep"></span><span>');
  const node = el(`<article class="card">${chip(card)}
    <div class="thumbrow">
      <a class="thumb" href="${esc(card.url)}" rel="noopener" target="_blank">
        ${card.thumbnail ? `<img src="${img(card.thumbnail)}" alt="" loading="lazy">` : ""}
        ${b.duration ? `<span class="duration">${esc(b.duration)}</span>` : ""}
        <span class="play">${PLAY}</span>
      </a>
      <div class="body">${siteRow(card)}${title(card)}${snippet(card)}
        <div class="meta"><span>${meta}</span></div>
      </div>
    </div>
  </article>`);
  if (b.embed_url) {
    const thumb = node.querySelector(".thumb");
    thumb.addEventListener("click", (e) => {
      e.preventDefault();
      const wrap = el(`<div class="embed"></div>`);
      const frame = document.createElement("iframe");
      frame.src = b.embed_url;
      frame.allow = "autoplay; encrypted-media; picture-in-picture";
      frame.allowFullscreen = true;
      wrap.appendChild(frame);
      node.querySelector(".thumbrow").replaceWith(wrap, el(`<div>${title(card)}</div>`));
    });
  }
  return node;
}

function product(card) {
  const b = card.body || {};
  const price = b.price ? `<span class="price">${esc(b.currency === "USD" ? "$" : (b.currency || "")) }${esc(b.price)}</span>` : "";
  const rating = b.rating ? `<span class="stars" title="${esc(b.rating)}">${stars(b.rating)}</span> <span>${num(b.rating_count)}</span>` : "";
  const meta = [b.availability, b.merchant].filter(Boolean).map(esc).join('</span><span class="sep"></span><span>');
  return el(`<article class="card">${chip(card)}
    <div class="thumbrow">
      ${card.thumbnail ? `<a class="thumb square" href="${esc(card.url)}" rel="noopener" target="_blank"><img src="${img(card.thumbnail)}" alt="" loading="lazy"></a>` : ""}
      <div class="body">${siteRow(card)}${title(card)}
        <div class="meta">${price} ${rating}</div>
        <div class="meta"><span>${meta}</span></div>
        ${snippet(card)}
      </div>
    </div>
  </article>`);
}

function book(card) {
  const b = card.body || {};
  const meta = [
    (b.authors || []).join(", "),
    b.year, b.pages ? b.pages + " pp" : "",
  ].filter(Boolean).map(esc).join('</span><span class="sep"></span><span>');
  const rating = b.rating ? `<span class="stars">${stars(b.rating)}</span> <span>${esc(b.rating)} · ${num(b.rating_count)} ratings</span>` : "";
  return el(`<article class="card">${chip(card)}
    <div class="thumbrow">
      ${card.thumbnail ? `<a class="thumb book" href="${esc(card.url)}" rel="noopener" target="_blank"><img src="${img(card.thumbnail)}" alt="" loading="lazy"></a>` : ""}
      <div class="body">${siteRow(card)}${title(card)}
        <div class="meta"><span>${meta}</span></div>
        <div class="meta">${rating}</div>
        ${snippet(card)}
      </div>
    </div>
  </article>`);
}

function article(card) {
  const b = card.body || {};
  const mins = b.word_count ? Math.max(1, Math.round(b.word_count / 220)) + " min read" : "";
  const meta = [b.author, b.published, mins].filter(Boolean).map(esc).join('</span><span class="sep"></span><span>');
  return el(`<article class="card">${chip(card)}${siteRow(card)}${title(card)}
    ${b.excerpt ? `<p class="snippet">${esc(b.excerpt)}</p>` : snippet(card)}
    <div class="meta"><span>${meta}</span></div>
  </article>`);
}

function weather(card) {
  const b = card.body || {};
  const days = (b.forecast || []).map((d) => `
    <div class="wx-day" title="${esc(d.condition || "")}">
      <div class="d">${esc(shortDay(d.date))}</div>
      ${icon(d.icon, 20)}
      <span class="hi">${Math.round(d.hi_c)}°</span> <span>${Math.round(d.lo_c)}°</span>
    </div>`).join("");
  return el(`<article class="card">${chip(card)}
    <div class="site">${esc(b.place)}</div>
    <div class="wx-now">
      <span class="icon">${icon(b.icon, 46)}</span>
      <span class="wx-temp">${Math.round(b.temp_c)}<sup>°C</sup></span>
      <div class="wx-cond">${esc(b.condition || "")}<br>
        ${b.wind_kmh ? `wind ${Math.round(b.wind_kmh)} km/h` : ""} ${b.humidity ? `· humidity ${b.humidity}%` : ""}
      </div>
    </div>
    ${days ? `<div class="wx-days">${days}</div>` : ""}
  </article>`);
}

function shortDay(date) {
  const d = new Date(date + "T00:00:00");
  return isNaN(d) ? date : d.toLocaleDateString(undefined, { weekday: "short" });
}

function definition(card) {
  const b = card.body || {};
  let out = "", lastPos = "";
  for (const s of b.senses || []) {
    if (s.part_of_speech && s.part_of_speech !== lastPos) {
      lastPos = s.part_of_speech;
      out += `<div class="def-pos">${esc(lastPos)}</div>`;
    }
    out += `<p class="def-sense">${esc(s.meaning)}${s.example ? ` <span class="def-ex">"${esc(s.example)}"</span>` : ""}</p>`;
  }
  return el(`<article class="card">${chip(card)}
    <p class="def-word">${esc(b.word || card.title)}<span class="def-phon">${esc(b.phonetic || "")}</span></p>
    ${out}
  </article>`);
}

function entity(card) {
  const b = card.body || {};
  const facts = (b.facts || []).map((f) =>
    `<tr><td class="k">${esc(f.label)}</td><td>${esc(f.value)}</td></tr>`).join("");
  return el(`<article class="card entity">${chip(card)}
    ${b.image ? `<img class="hero" src="${img(b.image)}" alt="" loading="lazy">` : ""}
    ${title(card)}
    <p class="desc">${esc(b.description || "")}</p>
    ${facts ? `<table>${facts}</table>` : ""}
    ${b.attribution ? `<div class="attr">${esc(b.attribution)}</div>` : ""}
  </article>`);
}

function qa(card) {
  const b = card.body || {};
  return el(`<article class="card">${chip(card)}${siteRow(card)}${title(card)}
    ${b.answer ? `<p class="snippet">${esc(strip(b.answer)).slice(0, 280)}</p>` : ""}
    <div class="meta"><span class="votes">▲ ${num(b.votes)}</span><span class="sep"></span><span>${num(b.comments)} comments</span></div>
  </article>`);
}

function post(card) {
  const b = card.body || {};
  return el(`<article class="card">${chip(card)}
    <div class="site"><b>${esc(b.author || "")}</b>&nbsp;<span class="handle">${esc(b.handle || "")}</span></div>
    <p class="snippet" style="color:var(--text)">${esc(b.text || "")}</p>
    <div class="meta"><span>${num(b.likes)} likes</span><span class="sep"></span><span>${num(b.reposts)} reposts</span>${b.published ? `<span class="sep"></span><span>${esc(b.published)}</span>` : ""}</div>
  </article>`);
}

function repo(card) {
  const b = card.body || {};
  return el(`<article class="card">${chip(card)}${siteRow(card)}${title(card)}
    ${b.description ? `<p class="snippet">${esc(b.description)}</p>` : ""}
    <div class="meta">${b.language ? `<span><span class="lang-dot"></span>${esc(b.language)}</span><span class="sep"></span>` : ""}<span>★ ${num(b.stars)}</span></div>
  </article>`);
}

function image(card) {
  const b = card.body || {};
  return el(`<a class="imgitem" href="${esc(b.source_page || card.url)}" rel="noopener" target="_blank" title="${esc(card.title)}">
    <img src="${img(card.thumbnail || card.url)}" alt="${esc(card.title)}" loading="lazy">
  </a>`);
}

function place(card) {
  const b = card.body || {};
  const meta = [b.address, b.hours].filter(Boolean).map(esc).join('</span><span class="sep"></span><span>');
  return el(`<article class="card">${chip(card)}${siteRow(card)}${title(card)}
    <div class="meta">${b.rating ? `<span class="stars">${stars(b.rating)}</span><span>${esc(b.rating)}</span><span class="sep"></span>` : ""}<span>${meta}</span></div>
  </article>`);
}

function strip(html) {
  const d = document.createElement("div");
  d.innerHTML = html;
  return d.textContent || "";
}

/* ---------- chart ---------- */

const SERIES_VARS = ["--chart-1", "--chart-2"];

function chart(card) {
  const b = card.body || {};
  const node = el(`<article class="card">${chip(card)}${siteRow(card)}${title(card)}${snippet(card)}
    <div class="chart"></div>
  </article>`);
  const holder = node.querySelector(".chart");
  // Colors come from tokens at render time so a theme flip redraws right.
  const draw = () => {
    holder.innerHTML = "";
    drawChart(holder, b);
  };
  requestAnimationFrame(draw);
  new MutationObserver(draw).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return node;
}

function drawChart(holder, b) {
  const css = getComputedStyle(document.documentElement);
  const colors = SERIES_VARS.map((v) => css.getPropertyValue(v).trim());
  const series = (b.series || []).slice(0, 2).filter((s) => (s.points || []).length > 0);
  if (!series.length) return;

  const spark = b.chart_kind === "spark";
  const W = 640, H = spark ? 64 : 190;
  const pad = spark ? { t: 6, r: 6, b: 6, l: 6 } : { t: 10, r: 12, b: 22, l: 40 };
  const all = series.flatMap((s) => s.points);
  let lo = Math.min(...all), hi = Math.max(...all);
  if (lo === hi) { lo -= 1; hi += 1; }
  const span = hi - lo;
  lo -= span * 0.06; hi += span * 0.06;

  const nPoints = Math.max(...series.map((s) => s.points.length));
  const x = (i) => pad.l + (i / Math.max(nPoints - 1, 1)) * (W - pad.l - pad.r);
  const y = (v) => pad.t + (1 - (v - lo) / (hi - lo)) * (H - pad.t - pad.b);

  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  svg.setAttribute("role", "img");

  if (!spark) {
    // Three recessive gridlines with value labels in muted ink.
    for (let g = 0; g < 3; g++) {
      const v = lo + ((g + 0.5) / 3) * (hi - lo);
      const line = document.createElementNS(svgNS, "line");
      line.setAttribute("class", "grid");
      line.setAttribute("x1", pad.l); line.setAttribute("x2", W - pad.r);
      line.setAttribute("y1", y(v)); line.setAttribute("y2", y(v));
      svg.appendChild(line);
      const label = document.createElementNS(svgNS, "text");
      label.setAttribute("class", "axis-label");
      label.setAttribute("x", pad.l - 6); label.setAttribute("y", y(v) + 4);
      label.setAttribute("text-anchor", "end");
      label.textContent = fmtVal(v) + (b.unit || "");
      svg.appendChild(label);
    }
    // Sparse x labels: first, middle, last non-empty.
    const labels = b.x_labels || [];
    const picks = [0, Math.floor((nPoints - 1) / 2), nPoints - 1];
    for (const i of picks) {
      if (!labels[i]) continue;
      const t = document.createElementNS(svgNS, "text");
      t.setAttribute("class", "axis-label");
      t.setAttribute("x", x(i));
      t.setAttribute("y", H - 6);
      t.setAttribute("text-anchor", i === 0 ? "start" : i === nPoints - 1 ? "end" : "middle");
      t.textContent = labels[i];
      svg.appendChild(t);
    }
  }

  if (b.chart_kind === "bar" && series.length === 1) {
    const pts = series[0].points;
    const bw = Math.max(2, (W - pad.l - pad.r) / pts.length - 2); // 2px surface gap
    const base = y(Math.max(lo, 0));
    pts.forEach((v, i) => {
      const rect = document.createElementNS(svgNS, "rect");
      rect.setAttribute("class", "bar");
      rect.setAttribute("x", x(i) - bw / 2);
      rect.setAttribute("y", Math.min(y(v), base));
      rect.setAttribute("width", bw);
      rect.setAttribute("height", Math.max(Math.abs(base - y(v)), 1));
      rect.setAttribute("fill", colors[0]);
      svg.appendChild(rect);
    });
  } else {
    series.forEach((s, si) => {
      const d = s.points.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join("");
      if (si === 0 && !spark) {
        const area = document.createElementNS(svgNS, "path");
        area.setAttribute("class", "area");
        area.setAttribute("fill", colors[0]);
        area.setAttribute("d", `${d}L${x(s.points.length - 1)},${y(lo)}L${x(0)},${y(lo)}Z`);
        svg.appendChild(area);
      }
      const path = document.createElementNS(svgNS, "path");
      path.setAttribute("class", "line");
      path.setAttribute("stroke", colors[si]);
      path.setAttribute("d", d);
      svg.appendChild(path);
    });
  }

  holder.appendChild(svg);

  // A legend only when there are two series; one series is named by the title.
  if (series.length > 1 && !spark) {
    holder.insertAdjacentElement("afterend", el(`<div class="chart-legend">${
      series.map((s, i) => `<span><span class="chip" style="background:${colors[i]}"></span>${esc(s.name || "series " + (i + 1))}</span>`).join("")
    }</div>`));
  }

  if (!spark) hoverLayer(holder, svg, { series, colors, labels: b.x_labels || [], x, y, nPoints, unit: b.unit || "", W, H, pad });
}

function fmtVal(v) {
  const a = Math.abs(v);
  if (a >= 1000) return Math.round(v).toLocaleString();
  if (a >= 100) return v.toFixed(0);
  return (Math.round(v * 10) / 10).toString();
}

// Crosshair plus tooltip; hover targets are the whole plot, not the 2px line.
function hoverLayer(holder, svg, ctx) {
  const svgNS = "http://www.w3.org/2000/svg";
  const cross = document.createElementNS(svgNS, "line");
  cross.setAttribute("class", "crosshair");
  cross.setAttribute("y1", ctx.pad.t);
  cross.setAttribute("y2", ctx.H - ctx.pad.b);
  cross.style.display = "none";
  svg.appendChild(cross);

  const dots = ctx.series.map((s, i) => {
    const c = document.createElementNS(svgNS, "circle");
    c.setAttribute("r", 4);
    c.setAttribute("fill", ctx.colors[i]);
    c.setAttribute("stroke", "var(--surface)");
    c.setAttribute("stroke-width", 2);
    c.style.display = "none";
    svg.appendChild(c);
    return c;
  });

  const tip = el(`<div class="chart-tip"></div>`);
  holder.appendChild(tip);

  svg.addEventListener("pointermove", (e) => {
    const rect = svg.getBoundingClientRect();
    const px = ((e.clientX - rect.left) / rect.width) * ctx.W;
    const i = Math.max(0, Math.min(ctx.nPoints - 1,
      Math.round(((px - ctx.pad.l) / (ctx.W - ctx.pad.l - ctx.pad.r)) * (ctx.nPoints - 1))));
    cross.style.display = "";
    cross.setAttribute("x1", ctx.x(i));
    cross.setAttribute("x2", ctx.x(i));
    let rows = "";
    ctx.series.forEach((s, si) => {
      const v = s.points[i];
      if (v == null) { dots[si].style.display = "none"; return; }
      dots[si].style.display = "";
      dots[si].setAttribute("cx", ctx.x(i));
      dots[si].setAttribute("cy", ctx.y(v));
      rows += `<div class="v"><span class="chip" style="background:${ctx.colors[si]}"></span>${esc(s.name || "")} ${fmtVal(v)}${esc(ctx.unit)}</div>`;
    });
    tip.innerHTML = `<div class="t">${esc(ctx.labels[i] || "#" + (i + 1))}</div>${rows}`;
    tip.style.display = "block";
    const tipX = (ctx.x(i) / ctx.W) * rect.width;
    tip.style.left = Math.min(tipX + 12, rect.width - tip.offsetWidth - 4) + "px";
    tip.style.top = "8px";
  });
  svg.addEventListener("pointerleave", () => {
    cross.style.display = "none";
    tip.style.display = "none";
    dots.forEach((d) => (d.style.display = "none"));
  });
}

export const renderers = {
  web, video, image, article, product, book, weather, chart,
  entity, definition, qa, post, repo, place,
};

export function renderCard(card) {
  const fn = renderers[card.kind] || web;
  try {
    return fn(card);
  } catch (err) {
    console.error("render", card.kind, err);
    return web(card);
  }
}
