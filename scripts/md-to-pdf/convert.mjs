#!/usr/bin/env node
import fs from 'fs';
import path from 'path';
import { execFileSync } from 'child_process';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const args = process.argv.slice(2);
const input = args.find((a) => !a.startsWith('-'));
const outFlagIdx = args.indexOf('-o');
const outHtml = outFlagIdx >= 0 ? args[outFlagIdx + 1] : undefined;
const wantPdf = args.includes('--pdf') || args.includes('--pdf-only');
const pdfOnly = args.includes('--pdf-only');

if (!input) {
  console.error('Uso: node convert.mjs <input.md> [-o salida.html] [--pdf | --pdf-only]');
  process.exit(1);
}

const md = fs.readFileSync(input, 'utf8');
const { Marked } = await import('marked');
const marked = new Marked();

const escapeHtml = (s) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

marked.use({
  renderer: {
    code(token, infostring) {
      const text = typeof token === 'string' ? token : token.text;
      const lang = typeof token === 'string' ? infostring : token.lang;
      if ((lang || '').toLowerCase() === 'mermaid') {
        return `<pre class="mermaid">${escapeHtml(text)}</pre>\n`;
      }
      const cls = lang ? ` class="language-${lang}"` : '';
      return `<pre><code${cls}>${escapeHtml(text)}</code></pre>\n`;
    },
  },
});

const body = marked.parse(md);

const title = path.basename(input, path.extname(input));

const css = `
  body { font-family: -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif;
         line-height: 1.6; color: #1f2328; max-width: 900px; margin: 0 auto;
         padding: 24px 40px; background: #fff; font-size: 15px; }
  h1, h2, h3, h4 { line-height: 1.25; margin: 1.5em 0 0.5em; }
  h1 { border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; }
  h2 { border-bottom: 1px solid #d0d7de; padding-bottom: 0.3em; margin-top: 2em; }
  code { background: #f6f8fa; border-radius: 4px; padding: 0.15em 0.4em;
         font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; }
  pre { background: #f6f8fa; border-radius: 6px; padding: 12px 16px; overflow-x: auto; }
  pre code { background: none; padding: 0; }
  pre.mermaid { background: #fff; text-align: center; padding: 0; border: none; }
  pre.mermaid svg { max-width: 100%; height: auto; }
  table { border-collapse: collapse; width: 100%; margin: 1em 0; font-size: 14px; }
  th, td { border: 1px solid #d0d7de; padding: 6px 12px; text-align: left; }
  th { background: #f6f8fa; }
  blockquote { border-left: 4px solid #d0d7de; margin: 1em 0; padding: 0.2em 1em; color: #57606a; }
  img { max-width: 100%; }
  hr { border: none; border-top: 1px solid #d0d7de; margin: 2em 0; }
  @media print {
    @page { margin: 1.6cm; }
    body { max-width: none; padding: 0; font-size: 12px; }
    pre, pre.mermaid, table, blockquote { break-inside: avoid; }
    h1, h2, h3 { break-after: avoid; }
  }
`;

async function inlineMermaid() {
  const cache = path.resolve(__dirname, '.cache', 'mermaid.min.js');
  if (fs.existsSync(cache)) return fs.readFileSync(cache, 'utf8');
  const url = 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js';
  const res = await fetch(url);
  if (!res.ok) throw new Error(`descarga de mermaid falló (${res.status})`);
  const js = await res.text();
  fs.mkdirSync(path.dirname(cache), { recursive: true });
  fs.writeFileSync(cache, js);
  return js;
}

let mermaidScript;
try {
  mermaidScript = await inlineMermaid();
} catch (e) {
  console.warn(`[aviso] ${e.message} — los diagramas no se van a renderizar. Verificá la conexión y volvé a correr.`);
  mermaidScript = '';
}

const html = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>${title}</title>
<style>${css}</style>
</head>
<body>
${body}
<script>${mermaidScript}</script>
<script>mermaid.initialize({ startOnLoad: true });</script>
</body>
</html>
`;

const outputDir = outHtml ? path.dirname(path.resolve(outHtml)) : path.resolve(__dirname, '..', '..', 'build');
fs.mkdirSync(outputDir, { recursive: true });

if (!pdfOnly) {
  const htmlPath = outHtml ? path.resolve(outHtml) : path.join(outputDir, `${title}.html`);
  fs.writeFileSync(htmlPath, html);
  console.log(`HTML: ${htmlPath}`);
}

if (wantPdf) {
  const candidates = [
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
  ];
  const chrome = candidates.find((c) => fs.existsSync(c));
  if (!chrome) {
    console.error('No se encontró Chrome/Chromium/Edge para generar el PDF. Abrí el HTML y usá Imprimir → PDF.');
    process.exit(1);
  }
  const tmpHtml = path.join(outputDir, `.${title}.tmp.html`);
  fs.writeFileSync(tmpHtml, html);
  const pdfPath = path.join(outputDir, `${title}.pdf`);
  try {
    execFileSync(chrome, [
      '--headless=new',
      '--disable-gpu',
      '--no-sandbox',
      '--run-all-compositor-stages-before-draw',
      '--virtual-time-budget=20000',
      `--print-to-pdf=${pdfPath}`,
      '--no-pdf-header-footer',
      `file://${tmpHtml}`,
    ], { stdio: 'inherit' });
    console.log(`PDF:  ${pdfPath}`);
  } finally {
    fs.rmSync(tmpHtml, { force: true });
  }
}
