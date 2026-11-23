import { marked } from 'marked';
import DOMPurify from 'dompurify';

marked.setOptions({ gfm: true, breaks: true });

// Content is written by squadron members: parse Markdown, then sanitize the
// resulting HTML. Raw HTML in the source is not trusted.
export function renderMarkdown(source) {
  const html = marked.parse(source || '');
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['target', 'rel'],
  });
}
