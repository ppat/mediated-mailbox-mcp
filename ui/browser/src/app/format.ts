// The formatting rules of docs/UI.md section 11. Each is a pure function from a value to its text, so
// every surface writes a count, a share, a time or an identifier the same way.

const counts = new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 });

// count writes a count with a thousands separator and no abbreviation, 12,480.
export function count(n: number): string {
  return counts.format(n);
}

// counted writes a count with its noun, singular for one, 1 rule and 12,480 messages. plural is the
// noun's plural where it is not the noun with an s.
export function counted(n: number, noun: string, plural = `${noun}s`): string {
  return `${count(n)} ${n === 1 ? noun : plural}`;
}

// share writes part of whole as a percent with one decimal, 14.8%. A share of nothing is 0.0%.
export function share(part: number, whole: number): string {
  return `${(whole === 0 ? 0 : (part / whole) * 100).toFixed(1)}%`;
}

// rate writes a current rate against its target with one decimal and the unit, 3.1 of 5.0 units/s.
export function rate(current: number, target: number): string {
  return `${current.toFixed(1)} of ${target.toFixed(1)} units/s`;
}

const units: [string, number][] = [
  ["d", 86_400_000],
  ["h", 3_600_000],
  ["m", 60_000],
  ["s", 1_000],
];

// duration writes a span in its largest two units, leaving out a second unit that is zero, so 1h 10m,
// 6h 41m, 12s and 14d. A span under a second is 0s.
export function duration(ms: number): string {
  const whole = Math.max(0, Math.floor(ms / 1_000) * 1_000);
  const first = units.findIndex(([, size]) => whole >= size);
  if (first < 0) {
    return "0s";
  }
  const [name, size] = units[first] ?? ["s", 1_000];
  const parts = [`${Math.floor(whole / size)}${name}`];
  const next = units[first + 1];
  if (next !== undefined) {
    const rest = Math.floor((whole % size) / next[1]);
    if (rest > 0) {
      parts.push(`${rest}${next[0]}`);
    }
  }
  return parts.join(" ");
}

// age writes the time since an instant in the same units, relative to now.
export function age(iso: string, now: number): string {
  return duration(now - Date.parse(iso));
}

// ageOf writes an age against a maximum, 1d 4h of 14d.
export function ageOf(iso: string, maximumMs: number, now: number): string {
  return `${age(iso, now)} of ${duration(maximumMs)}`;
}

// utc writes an instant as its UTC date and time to the minute with the Z suffix, 2026-09-10 10:12Z.
export function utc(iso: string): string {
  const t = new Date(Date.parse(iso)).toISOString();
  return `${t.slice(0, 10)} ${t.slice(11, 16)}Z`;
}

// utcTime writes an instant's UTC time alone, for a day already named, 10:12Z.
export function utcTime(iso: string): string {
  return `${new Date(Date.parse(iso)).toISOString().slice(11, 16)}Z`;
}

// utcDate writes an instant's UTC date alone, 2026-09-10.
export function utcDate(iso: string): string {
  return new Date(Date.parse(iso)).toISOString().slice(0, 10);
}

// local writes an instant in the browser's own time zone, for the hover of a UTC time.
export function local(iso: string): string {
  return new Date(Date.parse(iso)).toLocaleString();
}

// planId writes a plan's UUID as its first six characters with an ellipsis. The full identifier is
// shown on hover and in the address line.
export function planId(id: string): string {
  return id.length > 6 ? `${id.slice(0, 6)}…` : id;
}

// labelChange writes a label change as the two sets in brackets joined by an arrow,
// [INBOX] → [INBOX, Finance/Statements].
export function labelChange(before: readonly string[], after: readonly string[]): string {
  return `[${before.join(", ")}] → [${after.join(", ")}]`;
}
