const allowedTags = new Set([
  'A', 'B', 'BLOCKQUOTE', 'BR', 'CODE', 'DEL', 'EM', 'H1', 'H2', 'H3', 'H4',
  'HR', 'I', 'LI', 'OL', 'P', 'PRE', 'S', 'STRONG', 'TABLE', 'TBODY', 'TD',
  'TH', 'THEAD', 'TR', 'U', 'UL'
])

const discardTags = new Set([
  'BASE', 'BUTTON', 'EMBED', 'FORM', 'IFRAME', 'INPUT', 'LINK', 'MATH', 'META',
  'OBJECT', 'SCRIPT', 'SELECT', 'STYLE', 'SVG', 'TEMPLATE', 'TEXTAREA'
])

function safeLink(value: string): string | null {
  try {
    const url = new URL(value, window.location.href)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : null
  } catch {
    return null
  }
}

export function sanitizeAnnouncementHtml(html: string): string {
  const template = document.createElement('template')
  template.innerHTML = html

  const elements = Array.from(template.content.querySelectorAll('*')).reverse()
  for (const element of elements) {
    if (discardTags.has(element.tagName)) {
      element.remove()
      continue
    }
    if (!allowedTags.has(element.tagName)) {
      element.replaceWith(...Array.from(element.childNodes))
      continue
    }

    const originalHref = element.tagName === 'A'
      ? (element as HTMLAnchorElement).getAttribute('href') || ''
      : ''
    for (const attribute of Array.from(element.attributes)) {
      element.removeAttribute(attribute.name)
    }
    if (element.tagName === 'A') {
      const href = safeLink(originalHref)
      if (href) {
        element.setAttribute('href', href)
        element.setAttribute('rel', 'noopener noreferrer')
      } else {
        element.removeAttribute('href')
      }
    }
  }

  return template.innerHTML
}
