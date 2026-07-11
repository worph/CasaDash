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

/** The |---|:---:| row that turns the line above it into a table header. */
const SEPARATOR_ROW = /^\|?(\s*:?-{2,}:?\s*\|)+(\s*:?-{2,}:?\s*)?\|?$/

/** Split a `| a | b |` line into cells and wrap each in `th`/`td`. */
function row(line: string, cell: 'th' | 'td'): string {
  const cells = line
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((c) => `<${cell}>${inline(c.trim())}</${cell}>`)
  return '<tr>' + cells.join('') + '</tr>'
}

export interface MarkdownOptions {
  /** Keep single newlines as <br> (tips are line-oriented: credentials, URLs, paths). */
  breaks?: boolean
}

export function renderMarkdown(src: string, opts: MarkdownOptions = {}): string {
  const lines = escapeHtml(src).replace(/\r\n/g, '\n').split('\n')
  const out: string[] = []
  let para: string[] = []
  let inList = false
  let fence: string[] | null = null

  const flushPara = () => {
    if (para.length) {
      out.push('<p>' + inline(para.join(opts.breaks ? '<br>' : ' ')) + '</p>')
      para = []
    }
  }
  const flushList = () => {
    if (inList) {
      out.push('</ul>')
      inList = false
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]
    const l = raw.trim()
    if (fence) {
      if (l.startsWith('```')) {
        out.push('<pre><code>' + fence.join('\n') + '</code></pre>')
        fence = null
      } else {
        fence.push(raw)
      }
      continue
    }
    if (l.startsWith('```')) {
      flushPara()
      flushList()
      fence = []
      continue
    }
    if (!l) {
      flushPara()
      flushList()
      continue
    }
    // GFM table: a header row followed by a |---|:--:| delimiter row. Store tips
    // use these for credential tables, so they must survive.
    if (l.includes('|') && SEPARATOR_ROW.test((lines[i + 1] ?? '').trim())) {
      flushPara()
      flushList()
      out.push('<table><thead>' + row(l, 'th') + '</thead><tbody>')
      i++ // consume the delimiter row
      while (i + 1 < lines.length && lines[i + 1].trim().includes('|')) {
        out.push(row(lines[++i].trim(), 'td'))
      }
      out.push('</tbody></table>')
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
  if (fence) out.push('<pre><code>' + fence.join('\n') + '</code></pre>') // unterminated fence
  flushPara()
  flushList()
  return out.join('\n')
}
