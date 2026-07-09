// Minimal, XSS-safe markdown → HTML for store descriptions. HTML is escaped
// first, then only our own tags are emitted (links restricted to http/https),
// so the result is safe to use with Svelte's {@html}.

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function inline(s: string): string {
  return s
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`([^`]+?)`/g, '<code>$1</code>')
    .replace(
      /\[(.+?)\]\((https?:\/\/[^\s)]+)\)/g,
      '<a href="$2" target="_blank" rel="noreferrer">$1</a>',
    )
}

export function renderMarkdown(src: string): string {
  const lines = escapeHtml(src).replace(/\r\n/g, '\n').split('\n')
  const out: string[] = []
  let para: string[] = []
  let inList = false

  const flushPara = () => {
    if (para.length) {
      out.push('<p>' + inline(para.join(' ')) + '</p>')
      para = []
    }
  }
  const flushList = () => {
    if (inList) {
      out.push('</ul>')
      inList = false
    }
  }

  for (const raw of lines) {
    const l = raw.trim()
    if (!l) {
      flushPara()
      flushList()
      continue
    }
    let m: RegExpMatchArray | null
    if ((m = l.match(/^(#{1,4})\s+(.*)/))) {
      flushPara()
      flushList()
      const lvl = Math.min(6, m[1].length + 2)
      out.push(`<h${lvl}>${inline(m[2])}</h${lvl}>`)
      continue
    }
    if ((m = l.match(/^[-*]\s+(.*)/))) {
      flushPara()
      if (!inList) {
        out.push('<ul>')
        inList = true
      }
      out.push('<li>' + inline(m[1]) + '</li>')
      continue
    }
    para.push(l)
  }
  flushPara()
  flushList()
  return out.join('\n')
}
