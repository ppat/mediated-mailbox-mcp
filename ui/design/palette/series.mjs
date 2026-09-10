import { execSync } from 'node:child_process';
function oklchToHex(L, C, h) {
  const a = C * Math.cos(h * Math.PI / 180), b = C * Math.sin(h * Math.PI / 180);
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b, m_ = L - 0.1055613458 * a - 0.0638541728 * b, s_ = L - 0.0894841775 * a - 1.2914855480 * b;
  const l = l_ ** 3, m = m_ ** 3, s = s_ ** 3;
  const r = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, bb = -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s;
  const gam = x => { x = Math.min(1, Math.max(0, x)); return x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055; };
  return '#' + [r, g, bb].map(x => Math.round(gam(x) * 255).toString(16).padStart(2, '0')).join('');
}
const V = '/tmp/claude-10001/bundled-skills/2.1.266/b4f76e7ffde5602344a14c3e248c260f/dataviz/scripts/validate_palette.js';
const hues = { blue: 250, amber: 60, teal: 165, violet: 310, red: 25, olive: 100 };
const orders = [
  ['blue','amber','teal','violet','olive','red'],
  ['blue','amber','violet','teal','red','olive'],
  ['blue','amber','teal','red','violet','olive'],
  ['blue','olive','violet','amber','teal','red'],
  ['blue','amber','teal','violet','red','olive'],
  ['teal','red','blue','amber','violet','olive'],
];
for (const mode of ['dark','light']) {
  const L = mode==='dark'?0.64:0.55, C = mode==='dark'?0.13:0.13, surf = mode==='dark'?'#1f1c1a':'#fefdfb';
  for (const o of orders) {
    const hex = o.map(n => oklchToHex(L, C, hues[n]));
    let out=''; try { out = execSync(`node ${V} "${hex.join(',')}" --mode ${mode} --surface ${surf}`, {encoding:'utf8'}); } catch(e){ out = e.stdout; }
    const verdict = out.includes('ALL CHECKS PASS') ? 'PASS' : out.includes('FAILED') ? 'FAIL' : '?';
    const cvd = (out.match(/CVD separation\s+worst adjacent .*?ΔE ([\d.]+)/)||[])[1];
    const nv = (out.match(/Normal-vision floor\s+worst adjacent .*?ΔE ([\d.]+)/)||[])[1];
    const band = out.includes('[FAIL] Lightness band') ? 'BAND-FAIL' : '';
    console.log(mode, verdict, `cvd=${cvd} nv=${nv} ${band}`, o.join('>'), hex.join(','));
  }
}
