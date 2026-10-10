// Stand-in for the AI image endpoint in the GitHub Pages demo: draws a rainy
// poster from the prompt, deterministically, so the post flow can be tried
// without an OpenAI key. The real app calls gpt-image-1 on the server.

const PALETTES = [
  ['#0f766e', '#99f6e4', '#f0fdfa'],
  ['#1e3a8a', '#93c5fd', '#eff6ff'],
  ['#7c2d12', '#fdba74', '#fff7ed'],
  ['#4c1d95', '#c4b5fd', '#f5f3ff'],
  ['#334155', '#cbd5e1', '#f8fafc'],
]

function hash(text: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 0x01000193)
  return h >>> 0
}

/** Small seeded PRNG (mulberry32) so the same prompt always draws the same poster. */
function random(seed: number) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

const escape = (s: string) => s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

function wrap(text: string, width: number, maxLines: number): string[] {
  const lines: string[] = []
  let line = ''
  for (const word of text.split(/\s+/).filter(Boolean)) {
    if ((line + ' ' + word).trim().length > width) {
      lines.push(line)
      line = word
    } else line = (line + ' ' + word).trim()
  }
  if (line) lines.push(line)
  if (lines.length > maxLines) {
    lines.length = maxLines
    lines[maxLines - 1] = lines[maxLines - 1]!.replace(/.{0,3}$/, '…')
  }
  return lines
}

export function placeholderArt(prompt: string): string {
  const seed = hash(prompt.trim().toLowerCase())
  const rand = random(seed)
  const [dark, mid, light] = PALETTES[seed % PALETTES.length]!
  const clouds = Array.from({ length: 6 }, () => {
    const x = 80 + rand() * 860
    const y = 140 + rand() * 220
    const r = 70 + rand() * 90
    return `<circle cx="${x.toFixed(0)}" cy="${y.toFixed(0)}" r="${r.toFixed(0)}" fill="${light}" opacity="${(0.35 + rand() * 0.4).toFixed(2)}"/>`
  }).join('')
  const drops = Array.from({ length: 90 }, () => {
    const x = rand() * 1024
    const y = 300 + rand() * 560
    const len = 18 + rand() * 30
    return `<line x1="${x.toFixed(0)}" y1="${y.toFixed(0)}" x2="${(x - len * 0.3).toFixed(0)}" y2="${(y + len).toFixed(0)}"/>`
  }).join('')
  const lines = wrap(prompt.trim(), 34, 3)
    .map((l, i) => `<tspan x="64" dy="${i === 0 ? 0 : 52}">${escape(l)}</tspan>`)
    .join('')
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
<defs><linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${dark}"/><stop offset="1" stop-color="${mid}"/></linearGradient></defs>
<rect width="1024" height="1024" fill="url(#sky)"/>${clouds}
<g stroke="${light}" stroke-width="4" stroke-linecap="round" opacity="0.7">${drops}</g>
<rect y="820" width="1024" height="204" fill="${dark}" opacity="0.85"/>
<text x="64" y="890" font-family="Georgia, serif" font-size="44" fill="#fff">${lines}</text>
<text x="960" y="990" text-anchor="end" font-family="sans-serif" font-size="22" fill="${light}">Drizzle demo · placeholder, not AI</text>
</svg>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}
