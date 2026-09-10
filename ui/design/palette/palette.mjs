// oklch -> sRGB hex, plus WCAG contrast. Used to derive both ui palettes.
function oklchToHex(L, C, h) {
  const a = C * Math.cos(h * Math.PI / 180), b = C * Math.sin(h * Math.PI / 180);
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.2914855480 * b;
  const l = l_ ** 3, m = m_ ** 3, s = s_ ** 3;
  let r = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  let g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  let bb = -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s;
  const gam = x => { x = Math.min(1, Math.max(0, x)); return x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055; };
  const clip = [r, g, bb].some(x => x < -0.002 || x > 1.002);
  return { hex: '#' + [r, g, bb].map(x => Math.round(gam(x) * 255).toString(16).padStart(2, '0')).join(''), clip };
}
function lum(hex) {
  const c = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(v => v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}
function contrast(a, b) { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return ((x + 0.05) / (y + 0.05)).toFixed(2); }

// hue plan (oklch degrees): neutral warm 70 · action amber 55 · restricted red 25 · flagged violet 310 · ok green 150 · info blue 250
const spec = {
  dark: {
    ground:   [0.19, 0.006, 70], surface: [0.23, 0.006, 70], raised: [0.27, 0.007, 70], border: [0.34, 0.008, 70],
    text:     [0.90, 0.010, 80], muted:   [0.68, 0.012, 80], faint: [0.52, 0.010, 80],
    action:   [0.76, 0.130, 60], restricted: [0.72, 0.150, 25], flagged: [0.75, 0.110, 310], ok: [0.74, 0.120, 150], info: [0.75, 0.100, 250],
    chartfill:[0.45, 0.010, 70],
    s1: [0.72, 0.115, 250], s2: [0.75, 0.130, 60], s3: [0.74, 0.110, 165], s4: [0.75, 0.110, 310], s5: [0.72, 0.130, 25], s6: [0.78, 0.110, 100],
  },
  light: {
    ground:   [0.97, 0.006, 80], surface: [0.995, 0.003, 80], raised: [0.985, 0.005, 80], border: [0.86, 0.010, 75],
    text:     [0.24, 0.012, 60], muted:   [0.50, 0.015, 60], faint: [0.66, 0.012, 60],
    action:   [0.55, 0.120, 55], restricted: [0.52, 0.170, 25], flagged: [0.50, 0.140, 310], ok: [0.52, 0.130, 150], info: [0.52, 0.130, 250],
    chartfill:[0.82, 0.012, 75],
    s1: [0.52, 0.130, 250], s2: [0.58, 0.130, 55], s3: [0.55, 0.110, 165], s4: [0.52, 0.140, 310], s5: [0.55, 0.170, 25], s6: [0.62, 0.130, 100],
  },
};
const out = {};
for (const mode of ['dark', 'light']) {
  out[mode] = {};
  for (const [k, v] of Object.entries(spec[mode])) { const r = oklchToHex(...v); out[mode][k] = r.hex; if (r.clip) console.error(`clip: ${mode}.${k}`); }
}
console.log(JSON.stringify(out, null, 2));
for (const mode of ['dark', 'light']) {
  const p = out[mode];
  console.log(`\n== ${mode}: contrast vs surface ${p.surface} (AA text 4.5, UI 3.0)`);
  for (const k of ['text', 'muted', 'faint', 'action', 'restricted', 'flagged', 'ok', 'info', 'border', 's1', 's2', 's3', 's4', 's5', 's6'])
    console.log(`${k.padEnd(11)} ${p[k]}  surface ${contrast(p[k], p.surface)}  ground ${contrast(p[k], p.ground)}  raised ${contrast(p[k], p.raised)}`);
  console.log(`ground/surface ${contrast(p.ground, p.surface)}  surface/raised ${contrast(p.surface, p.raised)}  text on action ${contrast(p.text, p.action)}  ground-text on action ${contrast(p.ground, p.action)}`);
}
