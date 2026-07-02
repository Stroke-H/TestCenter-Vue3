import fs from 'fs';
import path from 'path';
import readline from 'readline';

const REPORT_DIR = process.env.SUBTITLE_REPORT_DIR || path.join('report', 'api_report', '.runtime', 'subtitle_manual');
const REPORT_ROOT = path.resolve(import.meta.dirname, '..', REPORT_DIR);
const SUMMARY_FILE = path.join(REPORT_ROOT, 'subtitle_summary.json');
const FAILURE_FILE = path.join(REPORT_ROOT, 'subtitle_failures.jsonl');
const REPORT_FILE = path.join(REPORT_ROOT, 'subtitle_report.html');

async function main() {
  const summary = readJSON(SUMMARY_FILE, {});
  const failures = await readFailures();
  const grouped = groupFailuresByCategory(failures);
  const html = renderReport(summary, grouped, failures);
  fs.writeFileSync(REPORT_FILE, html, 'utf8');
  console.log(`📄 字幕测试报告已生成: ${REPORT_FILE}`);
}

async function readFailures() {
  if (!fs.existsSync(FAILURE_FILE)) return [];
  const rows = [];
  const rl = readline.createInterface({
    input: fs.createReadStream(FAILURE_FILE, 'utf8'),
    crlfDelay: Infinity
  });
  for await (const line of rl) {
    if (!line.trim()) continue;
    rows.push(JSON.parse(line));
  }
  return rows;
}

function groupFailuresByCategory(failures) {
  const categoryMap = new Map();
  for (const failure of failures) {
    const category = failure.category || 'unknown';
    if (!categoryMap.has(category)) {
      categoryMap.set(category, new Map());
    }
    const map = categoryMap.get(category);
    const key = failure.dramaId || 'unknown';
    if (!map.has(key)) {
      map.set(key, {
        dramaId: failure.dramaId,
        intId: failure.intId,
        title: failure.title,
        cnName: failure.cnName,
        lang: failure.lang,
        failures: []
      });
    }
    map.get(key).failures.push(failure);
  }
  return categoryOrder().map(category => {
    const map = categoryMap.get(category.key) || new Map();
    return {
      ...category,
      groups: [...map.values()].sort((a, b) => String(a.intId || a.dramaId).localeCompare(String(b.intId || b.dramaId)))
    };
  }).filter(section => section.groups.length > 0);
}

function renderReport(summary, sections, failures) {
  const failed = Number(summary.failures || 0) > 0;
  const statusText = failed ? 'Failed' : 'Passed';
  const finishedAt = formatDateTime(summary.finishedAt || summary.generatedAt);
  const checkedText = `${summary.checkedFiles || 0}/${summary.totalCheckItems || summary.totalSubtitleUrls || 0}`;
  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>剧集外挂字幕测试报告</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f3f6fb;
      --panel: #ffffff;
      --text: #172033;
      --muted: #64748b;
      --line: #e6ebf2;
      --blue: #2563eb;
      --green: #059669;
      --red: #dc2626;
      --amber: #d97706;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif;
    }
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--bg); color: var(--text); }
    .hero { background: linear-gradient(135deg, #10213f, #173b72 55%, #0f766e); color: #fff; padding: 28px 30px 34px; }
    .hero-inner { max-width: 1240px; margin: 0 auto; display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
    h1 { margin: 0; font-size: 28px; line-height: 1.25; letter-spacing: 0; }
    .subtitle { margin-top: 10px; color: rgba(255,255,255,.78); font-size: 14px; line-height: 1.8; }
    .status { min-width: 148px; border: 1px solid rgba(255,255,255,.28); border-radius: 8px; padding: 14px 16px; background: rgba(255,255,255,.12); text-align: right; }
    .status-label { font-size: 12px; color: rgba(255,255,255,.72); text-transform: uppercase; }
    .status-value { margin-top: 5px; font-size: 26px; font-weight: 800; color: ${failed ? '#fecaca' : '#bbf7d0'}; }
    .page { max-width: 1240px; margin: -22px auto 36px; padding: 0 24px; }
    .metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
    .metric { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 16px; box-shadow: 0 8px 24px rgba(15, 23, 42, .06); }
    .metric-label { color: var(--muted); font-size: 13px; }
    .metric-value { margin-top: 8px; font-size: 26px; font-weight: 800; letter-spacing: 0; }
    .metric-note { margin-top: 5px; color: var(--muted); font-size: 12px; }
    .section { margin-top: 18px; background: var(--panel); border: 1px solid var(--line); border-radius: 8px; overflow: hidden; box-shadow: 0 8px 24px rgba(15, 23, 42, .04); }
    .section-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 18px; border-bottom: 1px solid var(--line); background: #fbfdff; }
    h2 { margin: 0; font-size: 18px; letter-spacing: 0; }
    .section-meta { color: var(--muted); font-size: 13px; }
    .summary-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; padding: 16px 18px; }
    .stat { border: 1px solid var(--line); border-radius: 8px; padding: 12px; background: #fff; }
    .stat-name { color: var(--muted); font-size: 12px; }
    .stat-value { margin-top: 6px; font-size: 20px; font-weight: 800; }
    .empty { padding: 30px 18px; color: var(--green); font-weight: 800; text-align: center; }
    .category-section { border-top: 1px solid var(--line); }
    .category-section:first-of-type { border-top: 0; }
    .category-section[open] .category-toggle { border-bottom: 1px solid var(--line); }
    .category-toggle { list-style: none; cursor: pointer; user-select: none; padding: 0; }
    .category-toggle::-webkit-details-marker { display: none; }
    .category-toggle .drama-head { border-top: 0; transition: background .18s ease; }
    .category-toggle:hover .drama-head { background: #eef6ff; }
    .toggle-title { display: flex; align-items: center; gap: 8px; }
    .toggle-icon { display: inline-grid; place-items: center; width: 22px; height: 22px; border-radius: 999px; background: #dbeafe; color: var(--blue); font-size: 13px; font-weight: 900; transition: transform .18s ease; }
    .category-section[open] .toggle-icon { transform: rotate(90deg); }
    .drama { border-top: 1px solid var(--line); }
    .drama:first-child { border-top: 0; }
    .drama-head { padding: 14px 18px; background: #f8fafc; display: grid; grid-template-columns: 1fr auto; gap: 12px; align-items: start; }
    .drama-title { font-weight: 800; line-height: 1.5; }
    .drama-sub { margin-top: 4px; color: var(--muted); font-size: 12px; line-height: 1.7; word-break: break-all; }
    .count-pill { display: inline-flex; align-items: center; height: 26px; padding: 0 10px; border-radius: 999px; background: #fee2e2; color: #991b1b; font-size: 12px; font-weight: 800; white-space: nowrap; }
    table { width: 100%; border-collapse: collapse; table-layout: fixed; }
    th, td { padding: 11px 12px; border-top: 1px solid #eef2f7; text-align: left; vertical-align: top; font-size: 13px; line-height: 1.55; }
    th { color: #475569; background: #ffffff; font-weight: 800; }
    .category { display: inline-flex; align-items: center; height: 24px; padding: 0 9px; border-radius: 999px; background: #dbeafe; color: #1d4ed8; font-weight: 800; white-space: nowrap; font-size: 12px; }
    .category.fetch, .category.missing_subtitle { background: #fee2e2; color: #991b1b; }
    .category.subtitle_count { background: #ffedd5; color: #9a3412; }
    .category.timestamp { background: #ede9fe; color: #6d28d9; }
    .cue { white-space: pre-wrap; word-break: break-word; color: #0f172a; max-height: 130px; overflow: auto; }
    .url { word-break: break-all; color: var(--blue); }
    .muted { color: var(--muted); }
    .back-top { position: fixed; right: 22px; bottom: 22px; z-index: 20; width: 46px; height: 46px; border: 0; border-radius: 999px; background: var(--blue); color: #fff; font-size: 20px; font-weight: 900; box-shadow: 0 12px 28px rgba(37, 99, 235, .32); cursor: pointer; }
    .back-top:hover { background: #1d4ed8; }
    .back-top:active { transform: translateY(1px); }
    @media (max-width: 900px) {
      .hero-inner { flex-direction: column; }
      .status { text-align: left; }
      .metrics, .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      table { table-layout: auto; }
      .back-top { right: 14px; bottom: 14px; }
    }
  </style>
</head>
<body id="top">
  <header class="hero">
    <div class="hero-inner">
      <div>
        <h1>剧集外挂字幕测试报告</h1>
        <div class="subtitle">生成时间：${escapeHTML(finishedAt)}<br>检查规则：字幕数量、缺失字幕、时间轴合法、10 分钟上限、字幕地址可访问性</div>
      </div>
      <div class="status">
        <div class="status-label">Result</div>
        <div class="status-value">${statusText}</div>
      </div>
    </div>
  </header>
  <main class="page">
    <section class="metrics">
      ${metric('在线剧集', summary.totalDramas, '本次测试环境返回的剧集数')}
      ${metric('外挂剧', summary.externalDramas, 'embedded_subtitle = 0')}
      ${metric('检查项', checkedText, '字幕文件、数量校验和缺失校验')}
      ${metric('异常条数', summary.failures || 0, failed ? '需要处理' : '未发现异常')}
    </section>
    <section class="section">
      <div class="section-head">
        <h2>检查概览</h2>
        <div class="section-meta">准备外挂剧 ${summary.preparedDramas || 0}/${summary.externalDramas || 0}</div>
      </div>
      <div class="summary-grid">
        ${stat('异常文件/剧集', summary.failedFiles || 0)}
        ${stat('正片字幕文件', summary.totalSubtitleUrls || 0)}
        ${stat('字幕数量异常', summary.subtitleCountFailures || 0)}
        ${stat('缺失字幕', summary.missingSubtitleFailures || 0)}
        ${stat('时间轴异常', summary.timestampFailures || 0)}
        ${stat('拉取失败', summary.fetchFailures || 0)}
        ${stat('去重任务', summary.uniqueTasks || summary.checkedFiles || 0)}
        ${stat('缓存命中', summary.cacheHits || 0)}
      </div>
    </section>
    <section class="section">
      <div class="section-head">
        <h2>异常明细</h2>
        <div class="section-meta">${failures.length ? `${failures.length} 条异常` : '全部通过'}</div>
      </div>
      ${sections.length === 0 ? '<div class="empty">未发现外挂字幕异常。</div>' : sections.map(renderCategorySection).join('')}
    </section>
  </main>
  <button class="back-top" type="button" aria-label="返回顶部" title="返回顶部">↑</button>
  <script>
    document.querySelector('.back-top')?.addEventListener('click', function () {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    });
  </script>
</body>
</html>`;
}

function renderCategorySection(section) {
  const failureCount = section.groups.reduce((sum, group) => sum + group.failures.length, 0);
  return `<details class="category-section">
    <summary class="category-toggle">
      <div class="drama-head">
      <div>
        <div class="drama-title toggle-title"><span class="toggle-icon">›</span>${escapeHTML(section.label)}</div>
        <div class="drama-sub">${escapeHTML(section.description)}</div>
      </div>
      <span class="count-pill">${section.groups.length} 部剧 / ${failureCount} 条</span>
      </div>
    </summary>
    ${section.groups.map(renderDramaGroup).join('')}
  </details>`;
}

function renderDramaGroup(group) {
  return `<article class="drama">
    <div class="drama-head">
      <div>
        <div class="drama-title">${escapeHTML(group.title || '-')} / ${escapeHTML(group.cnName || '-')}</div>
        <div class="drama-sub">ID: ${escapeHTML(group.dramaId || '-')}　IntID: ${escapeHTML(group.intId || '-')}　Lang: ${escapeHTML(group.lang || '-')}</div>
      </div>
      <span class="count-pill">${group.failures.length} 条异常</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width: 104px;">分类</th>
          <th style="width: 150px;">章节/时间</th>
          <th>问题</th>
          <th>异常字幕</th>
          <th>字幕地址</th>
        </tr>
      </thead>
      <tbody>${group.failures.map(renderFailureRow).join('')}</tbody>
    </table>
  </article>`;
}

function renderFailureRow(failure) {
  const cue = failure.cue || {};
  const timeText = cue.start && cue.end ? `${cue.start} --> ${cue.end}` : '-';
  const chapterText = failure.chapterIndex ? `第 ${escapeHTML(failure.chapterIndex)} 集<br>${escapeHTML(timeText)}` : '<span class="muted">未解析到章节字幕</span>';
  const evidence = formatEvidence(failure, cue);
  return `<tr>
    <td><span class="category ${escapeHTML(failure.category)}">${categoryLabel(failure.category)}</span></td>
    <td>${chapterText}</td>
    <td>${escapeHTML(failure.message || '-')}</td>
    <td><div class="cue">${escapeHTML(evidence)}</div></td>
    <td><div class="url">${escapeHTML(failure.url || '-')}</div></td>
  </tr>`;
}

function formatEvidence(failure, cue) {
  if (Array.isArray(cue.evidence) && cue.evidence.length > 0) {
    return cue.evidence.map(item => {
      const prefix = item.index ? `Cue ${item.index}: ` : '';
      const reason = item.reason ? `\n原因: ${item.reason}` : '';
      return `${prefix}${item.text || ''}${reason}`;
    }).join('\n\n');
  }
  if (failure.category === 'subtitle_count') {
    return `剧集集数: ${cue.expectedChapters ?? failure.dramaChapterCount ?? '-'}\n字幕文件数: ${cue.subtitleFileCount ?? failure.subtitleFileCount ?? '-'}`;
  }
  return cue.text || cue.reason || formatCueMeta(cue) || '-';
}

function metric(label, value, note) {
  return `<div class="metric"><div class="metric-label">${escapeHTML(label)}</div><div class="metric-value">${escapeHTML(value ?? 0)}</div><div class="metric-note">${escapeHTML(note || '')}</div></div>`;
}

function stat(label, value) {
  return `<div class="stat"><div class="stat-name">${escapeHTML(label)}</div><div class="stat-value">${escapeHTML(value ?? 0)}</div></div>`;
}

function categoryLabel(category) {
  return {
    timestamp: '时间轴',
    fetch: '拉取',
    subtitle_count: '数量',
    missing_subtitle: '缺失字幕'
  }[category] || category;
}

function categoryOrder() {
  return [
    { key: 'subtitle_count', label: '字幕数量异常', description: '剧集 chapters 数量与解析到的正片 VTT 字幕文件数不一致。' },
    { key: 'missing_subtitle', label: '缺失字幕', description: '外挂剧在章节数据中没有解析到正片 VTT 字幕地址。' },
    { key: 'timestamp', label: '时间轴异常', description: '字幕时间戳存在倒退、结束早于开始、格式异常或超过 10 分钟。' },
    { key: 'fetch', label: '拉取失败', description: '字幕地址请求失败，无法完成内容检查。' },
    { key: 'unknown', label: '其他异常', description: '未归入固定分类的异常。' }
  ];
}

function formatCueMeta(cue) {
  const fields = [];
  if (cue.start) fields.push(`开始: ${cue.start}`);
  if (cue.end) fields.push(`结束: ${cue.end}`);
  return fields.join('\n');
}

function formatDateTime(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

function readJSON(file, fallback) {
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch {
    return fallback;
  }
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, char => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[char]));
}

main().catch(error => {
  console.error('🔥 字幕报告生成失败:', error);
  process.exit(1);
});
