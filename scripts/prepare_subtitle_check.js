import fs from 'fs';
import path from 'path';

const EMAIL = process.env.EMAIL || '';
const PASSWORD = process.env.PASSWORD || '';
const LOGIN_URL = process.env.LOGIN_URL || 'http://35.225.224.94:8080/api/pwd_login';
const DRAMA_LIST_URL = process.env.DRAMA_LIST_URL || 'http://35.225.224.94:8080/api/management/drama/all_online_ids';
const REPORT_DIR = process.env.SUBTITLE_REPORT_DIR || path.join('report', 'api_report', '.runtime', 'subtitle_manual');
const REPORT_ROOT = path.resolve(import.meta.dirname, '..', REPORT_DIR);
const QUEUE_FILE = path.join(REPORT_ROOT, 'subtitle_queue.jsonl');
const SUMMARY_FILE = path.join(REPORT_ROOT, 'subtitle_summary.json');
const SINGLE_DRAMA_ID = String(process.env.SUBTITLE_CHECK_DRAMA_ID || '').trim();
const PAGE_SIZE = Number(process.env.SUBTITLE_DRAMA_PAGE_SIZE || 200);
const CHAPTER_PAGE_SIZE = Number(process.env.SUBTITLE_CHAPTER_PAGE_SIZE || 200);
const CHAPTER_CONCURRENCY = clamp(Number(process.env.SUBTITLE_CHAPTER_CONCURRENCY || 16), 1, 24);
const REQUEST_TIMEOUT_MS = Number(process.env.SUBTITLE_PREPARE_TIMEOUT_MS || 20000);
const FETCH_RETRY_ATTEMPTS = Number(process.env.SUBTITLE_PREPARE_RETRY_ATTEMPTS || 3);
const APP = process.env.SUBTITLE_CHAPTER_APP || 'com.company.shortsdrama.wave';
const RESOLUTION = process.env.SUBTITLE_CHAPTER_RESOLUTION || '720p';

async function main() {
  fs.mkdirSync(REPORT_ROOT, { recursive: true });
  fs.writeFileSync(QUEUE_FILE, '', 'utf8');

  if (!EMAIL || !PASSWORD) {
    throw new Error('缺少登录账号配置：请检查 DRAMA_* 环境变量');
  }

  console.log('🚀 开始准备剧集外挂字幕测试数据');
  console.log(`📁 报告目录: ${REPORT_ROOT}`);
  if (SINGLE_DRAMA_ID) {
    console.log(`🎯 单剧调试模式: ${SINGLE_DRAMA_ID}`);
  }

  const xToken = await login();
  const apiBase = DRAMA_LIST_URL.split('/api/')[0].replace(/\/+$/, '');
  const onlineIds = await fetchOnlineDramaIds(xToken);
  const dramaMeta = await fetchDramaMetadata(apiBase, xToken, new Set(onlineIds));
  const dramas = onlineIds.map(id => dramaMeta[id] || { id }).filter(item => item.id);
  const selected = SINGLE_DRAMA_ID ? dramas.filter(item => item.id === SINGLE_DRAMA_ID) : dramas;
  const externalDramas = selected.filter(item => String(item.embedded_subtitle) === '0');

  console.log(`🎬 在线剧集 ${selected.length} 部，外挂剧 ${externalDramas.length} 部，开始并发拉取章节列表，并发 ${CHAPTER_CONCURRENCY}`);
  const progress = {
    generatedAt: new Date().toISOString(),
    totalDramas: selected.length,
    externalDramas: externalDramas.length,
    preparedDramas: 0,
    totalSubtitleUrls: 0,
    chapterListFailures: 0,
    checkedFiles: 0,
    failedFiles: 0,
    failures: 0,
    timestampFailures: 0,
    fetchFailures: 0,
    missingSubtitleFailures: 0,
    subtitleCountFailures: 0,
    missingSubtitleDramas: 0,
    dramasWithSubtitle: 0,
    allSubtitleFallbackDramas: 0,
    allSubtitleFallbackUrls: 0,
    status: 'preparing'
  };
  writeSummary(progress);

  const writer = fs.createWriteStream(QUEUE_FILE, { flags: 'a', encoding: 'utf8' });
  let nextExternalProgressPercent = 10;
  let lastLoggedPreparedDramas = 0;
  let lastLoggedSubtitleUrls = 0;
  let lastLoggedDramasWithSubtitle = 0;
  let lastLoggedMissingSubtitleDramas = 0;
  let lastLoggedCountFailures = 0;
  let lastLoggedFallbackDramas = 0;
  let unchangedSubtitleLogCount = 0;
  await runPool(externalDramas, CHAPTER_CONCURRENCY, async drama => {
    const expectedChapterCount = Number(drama.chapters || 0);
    const chapters = await fetchChapterList(apiBase, xToken, drama.id);
    let entries = buildChapterSubtitleEntries(drama, chapters);
    if (expectedChapterCount > 0 && entries.length !== expectedChapterCount) {
      const fallbackEntries = await fetchAllSubtitleEntries(apiBase, xToken, drama);
      if (fallbackEntries.length > entries.length) {
        entries = fallbackEntries;
        progress.allSubtitleFallbackDramas += 1;
        progress.allSubtitleFallbackUrls += fallbackEntries.length;
      }
    }
    if (expectedChapterCount > 0 && entries.length !== expectedChapterCount) {
      writer.write(JSON.stringify({
        dramaId: drama.id,
        intId: drama.int_id,
        title: drama.title,
        cnName: drama.cn_name,
        lang: normalizeLang(drama.lang),
        rawLang: String(drama.lang || ''),
        embeddedSubtitle: drama.embedded_subtitle,
        dramaChapterCount: expectedChapterCount,
        subtitleFileCount: entries.length,
        chapterId: '',
        chapterIndex: '',
        url: '',
        subtitleCountMismatch: true
      }) + '\n');
      progress.subtitleCountFailures += 1;
    }
    for (const entry of entries) {
      writer.write(JSON.stringify(entry) + '\n');
    }
    if (entries.length === 0) {
      writer.write(JSON.stringify({
        dramaId: drama.id,
        intId: drama.int_id,
        title: drama.title,
        cnName: drama.cn_name,
        lang: normalizeLang(drama.lang),
        rawLang: String(drama.lang || ''),
        embeddedSubtitle: drama.embedded_subtitle,
        dramaChapterCount: drama.chapters,
        chapterId: '',
        chapterIndex: '',
        url: '',
        missingSubtitle: true
      }) + '\n');
      progress.missingSubtitleDramas += 1;
    } else {
      progress.dramasWithSubtitle += 1;
    }
    progress.preparedDramas += 1;
    progress.totalSubtitleUrls += entries.length;
    nextExternalProgressPercent = logExternalCollectionProgress(progress, selected.length, externalDramas.length, nextExternalProgressPercent);
    if (progress.preparedDramas % 50 === 0 || progress.preparedDramas === externalDramas.length) {
      writeSummary(progress);
      const processedDramas = progress.preparedDramas - lastLoggedPreparedDramas;
      const addedSubtitleUrls = progress.totalSubtitleUrls - lastLoggedSubtitleUrls;
      const addedSubtitleDramas = progress.dramasWithSubtitle - lastLoggedDramasWithSubtitle;
      const addedMissingDramas = progress.missingSubtitleDramas - lastLoggedMissingSubtitleDramas;
      const addedCountFailures = progress.subtitleCountFailures - lastLoggedCountFailures;
      const addedFallbackDramas = progress.allSubtitleFallbackDramas - lastLoggedFallbackDramas;
      const subtitleRate = progress.preparedDramas > 0 ? ((progress.dramasWithSubtitle / progress.preparedDramas) * 100).toFixed(1) : '0.0';
      const percent = externalDramas > 0 ? ((progress.preparedDramas / externalDramas) * 100).toFixed(1) : '100.0';
      lastLoggedPreparedDramas = progress.preparedDramas;
      lastLoggedSubtitleUrls = progress.totalSubtitleUrls;
      lastLoggedDramasWithSubtitle = progress.dramasWithSubtitle;
      lastLoggedMissingSubtitleDramas = progress.missingSubtitleDramas;
      lastLoggedCountFailures = progress.subtitleCountFailures;
      lastLoggedFallbackDramas = progress.allSubtitleFallbackDramas;
      if (addedSubtitleUrls === 0) unchangedSubtitleLogCount += 1;
      else unchangedSubtitleLogCount = 0;
      console.log(`进度 ${progress.preparedDramas}/${externalDramas.length}（${percent}%），本段处理外挂剧 ${processedDramas} 部，新增正片字幕 ${addedSubtitleUrls} 个，新增有字幕剧 ${addedSubtitleDramas} 部，新增 all_subtitle 兜底 ${addedFallbackDramas} 部，新增未解析 ${addedMissingDramas} 部，新增数量异常 ${addedCountFailures} 部，累计正片字幕 ${progress.totalSubtitleUrls} 个，有字幕剧 ${progress.dramasWithSubtitle} 部（命中率 ${subtitleRate}%），all_subtitle 兜底 ${progress.allSubtitleFallbackDramas} 部，未解析到字幕 ${progress.missingSubtitleDramas} 部，数量异常 ${progress.subtitleCountFailures} 部`);
      if (unchangedSubtitleLogCount > 0 && unchangedSubtitleLogCount % 10 === 0) {
        console.log(`ℹ️ 最近 ${unchangedSubtitleLogCount * 50} 部外挂剧未新增正片字幕，章节接口仍在继续推进；这通常表示当前排序区间内确实没有可解析的正片 VTT。`);
      }
    }
  });
  await closeWriter(writer);

  writeSummary({
    ...progress,
    preparedDramas: externalDramas.length,
    status: 'prepared'
  });

  console.log(`✅ 准备完成：在线剧集 ${selected.length} 部，外挂剧 ${externalDramas.length} 部，字幕文件 ${progress.totalSubtitleUrls} 个`);
}

async function login() {
  const response = await fetchWithRetry(LOGIN_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Connection: 'close' },
    body: JSON.stringify({ app: 'com.shorts.wave.drama', email: EMAIL, password: PASSWORD })
  }, '登录接口');
  const responseText = await response.text();
  if (!response.ok) {
    throw new Error(`登录失败 HTTP ${response.status}: ${responseText}`);
  }
  const cookie = response.headers.get('set-cookie') || '';
  const match = cookie.match(/x-token=([^;]+)/);
  if (!match?.[1]) {
    throw new Error(`登录接口未返回 x-token cookie: ${summarizeLoginResponse(responseText)}`);
  }
  console.log(`✅ 已获取 x-token: ${match[1].slice(0, 12)}...`);
  return match[1];
}

function summarizeLoginResponse(text) {
  try {
    const payload = JSON.parse(text);
    const code = payload?.code ?? '-';
    const message = payload?.msg || payload?.message || payload?.error || '-';
    return `code=${code}, msg=${message}`;
  } catch {
    return String(text || '').slice(0, 200) || 'empty response';
  }
}

async function fetchOnlineDramaIds(xToken) {
  const response = await fetchWithRetry(DRAMA_LIST_URL, { headers: { Cookie: `x-token=${xToken}`, Connection: 'close' } }, '在线剧集 ID 列表');
  if (!response.ok) {
    throw new Error(`在线剧集 ID 列表失败 HTTP ${response.status}: ${await response.text()}`);
  }
  const text = await response.text();
  const ids = parseOnlineDramaIds(text);
  console.log(`🎞️ 权威在线剧集 ID: ${ids.length} 条`);
  return [...new Set(ids)];
}

function parseOnlineDramaIds(text) {
  const content = String(text || '').trim();
  if (!content) return [];
  try {
    const payload = JSON.parse(content);
    const list = Array.isArray(payload) ? payload : extractListPayload(payload);
    return list.map(item => {
      if (typeof item === 'string') return item.trim();
      return String(item?._id || item?.id || item?.drama_id || '').trim();
    }).filter(isHexObjectId);
  } catch {
    const lines = content.split(/[\r\n]+/).filter(Boolean);
    if (lines.length > 1 && lines[0].includes(',')) {
      const headers = parseCSVLine(lines[0]).map(value => value.trim().toLowerCase());
      const idIndex = ['_id', 'id', 'drama_id'].map(key => headers.indexOf(key)).find(index => index >= 0) ?? 0;
      return lines.slice(1).map(line => (parseCSVLine(line)[idIndex] || '').trim()).filter(isHexObjectId);
    }
    return content.match(/[0-9a-fA-F]{24}/g) || [];
  }
}

async function fetchDramaMetadata(apiBase, xToken, allowedIds) {
  const dramas = {};
  let page = 1;
  let total = 0;
  while (true) {
    const url = `${apiBase}/api/management/drama/list?online=1&page=${page}&page_size=${PAGE_SIZE}`;
    const response = await fetchWithRetry(url, { headers: { Cookie: `x-token=${xToken}`, Connection: 'close' } }, `剧集元信息 page=${page}`);
    if (!response.ok) {
      throw new Error(`剧集元信息接口失败 page=${page} HTTP ${response.status}: ${await response.text()}`);
    }
    const payload = await response.json();
    const list = Array.isArray(payload?.data) ? payload.data : [];
    total = Number(payload?.total || total || 0);
    for (const item of list) {
      const id = String(item?._id || item?.id || item?.drama_id || '').trim();
      if (!/^[0-9a-fA-F]{24}$/.test(id)) continue;
      if (allowedIds?.size && !allowedIds.has(id)) continue;
      dramas[id] = {
        id,
        int_id: String(item?.int_id ?? item?.intId ?? ''),
        title: String(item?.title ?? item?.name ?? ''),
        cn_name: String(item?.cn_name ?? item?.cnName ?? item?.cn_title ?? ''),
        lang: String(item?.lang ?? item?.language ?? ''),
        embedded_subtitle: Number(item?.embedded_subtitle ?? item?.embeddedSubtitle),
        chapters: Number(item?.chapters ?? item?.chapter_count ?? item?.chapterCount ?? 0)
      };
    }
    if (SINGLE_DRAMA_ID && dramas[SINGLE_DRAMA_ID]) break;
    if (list.length === 0 || (total > 0 && page * PAGE_SIZE >= total)) break;
    page += 1;
  }
  return dramas;
}

function parseCSVLine(line) {
  const result = [];
  let current = '';
  let inQuotes = false;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    const next = line[i + 1];
    if (char === '"' && inQuotes && next === '"') {
      current += '"';
      i += 1;
      continue;
    }
    if (char === '"') {
      inQuotes = !inQuotes;
      continue;
    }
    if (char === ',' && !inQuotes) {
      result.push(current);
      current = '';
      continue;
    }
    current += char;
  }
  result.push(current);
  return result;
}

function isHexObjectId(value) {
  return /^[0-9a-fA-F]{24}$/.test(String(value || '').trim());
}

function extractListPayload(payload) {
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload?.data)) return payload.data;
  if (Array.isArray(payload?.data?.data)) return payload.data.data;
  if (Array.isArray(payload?.data?.list)) return payload.data.list;
  if (Array.isArray(payload?.data?.items)) return payload.data.items;
  if (Array.isArray(payload?.items)) return payload.items;
  if (Array.isArray(payload?.list)) return payload.list;
  if (Array.isArray(payload?.rows)) return payload.rows;
  return [];
}

async function fetchChapterList(apiBase, xToken, dramaId) {
  const chapters = [];
  let page = 1;
  let total = 0;
  while (true) {
    const params = new URLSearchParams({
      app: APP,
      drama_id: dramaId,
      page: String(page),
      page_size: String(CHAPTER_PAGE_SIZE),
      resolution: RESOLUTION
    });
    const url = `${apiBase}/api/management/drama/chapter/list?${params.toString()}`;
    const response = await fetchWithRetry(url, { headers: { Cookie: `x-token=${xToken}`, Connection: 'close' } }, `章节列表 drama=${dramaId} page=${page}`);
    if (!response.ok) {
      throw new Error(`章节接口失败 drama=${dramaId} page=${page} HTTP ${response.status}: ${await response.text()}`);
    }
    const payload = await response.json();
    const list = extractListPayload(payload);
    total = Number(payload?.total || payload?.data?.total || total || 0);
    chapters.push(...list);
    if (list.length === 0 || (total > 0 && page * CHAPTER_PAGE_SIZE >= total)) break;
    page += 1;
  }
  return chapters;
}

function buildChapterSubtitleEntries(drama, chapters) {
  return chapters
    .map((chapter, index) => ({ chapter, index }))
    .flatMap(({ chapter, index }) => extractSubtitleURLs(chapter).map(url => ({
      dramaId: drama.id,
      intId: drama.int_id,
      title: drama.title,
      cnName: drama.cn_name,
      lang: normalizeLang(drama.lang),
      rawLang: String(drama.lang || ''),
      embeddedSubtitle: drama.embedded_subtitle,
      dramaChapterCount: drama.chapters,
      chapterId: String(chapter?._id ?? chapter?.id ?? chapter?.chapter_id ?? ''),
      chapterIndex: Number(chapter?.index ?? chapter?.chapter_index ?? chapter?.episode ?? index + 1),
      url,
      subtitleSource: 'chapter_list'
    })));
}

async function fetchAllSubtitleEntries(apiBase, xToken, drama) {
  for (const language of getAllSubtitleLanguageCandidates(drama.lang)) {
    const url = `${apiBase}/api/management/drama/all_subtitle/${encodeURIComponent(drama.id)}/${encodeURIComponent(language)}?app=${encodeURIComponent(APP)}`;
    const response = await fetchWithRetry(url, { headers: { Cookie: `x-token=${xToken}`, Connection: 'close' } }, `全部字幕 drama=${drama.id} lang=${language}`);
    if (!response.ok) {
      throw new Error(`全部字幕接口失败 drama=${drama.id} lang=${language} HTTP ${response.status}: ${await response.text()}`);
    }
    const payload = await response.json();
    const list = extractListPayload(payload);
    const entries = list
      .map((item, index) => ({
        dramaId: drama.id,
        intId: drama.int_id,
        title: drama.title,
        cnName: drama.cn_name,
        lang: normalizeLang(drama.lang),
        rawLang: String(drama.lang || ''),
        embeddedSubtitle: drama.embedded_subtitle,
        dramaChapterCount: drama.chapters,
        chapterId: String(item?._id ?? item?.id ?? item?.chapter_id ?? ''),
        chapterIndex: Number(item?.index ?? item?.chapter_index ?? item?.episode ?? index + 1),
        url: String(item?.url || '').trim(),
        subtitleLanguage: String(item?.language || language),
        subtitleSource: 'all_subtitle'
      }))
      .filter(entry => isPlayableSubtitleURL(entry.url));
    if (entries.length > 0) {
      return dedupeSubtitleEntries(entries);
    }
  }
  return [];
}

function getAllSubtitleLanguageCandidates(lang) {
  const normalized = normalizeLang(lang);
  const languageMap = {
    en: ['eng-US'],
    es: ['spa-ES'],
    ko: ['kor-KR'],
    ja: ['jpn-JP'],
    fr: ['fra-FR'],
    id: ['ind-ID'],
    th: ['tha-TH'],
    de: ['deu-DE'],
    pt: ['por-PT', 'por-BR'],
    tr: ['tur-TR'],
    ar: ['ara-SA'],
    ru: ['rus-RU'],
    vi: ['vie-VN'],
    ms: ['msa-MY'],
    hi: ['hin-IN'],
    zh: ['zho-CN', 'cmn-CN'],
    'zh-cn': ['zho-CN', 'cmn-CN'],
    'zh-tw': ['zho-TW', 'chi-TW', 'zho-Hant', 'cmn-Hant', 'cht-TW'],
    'zh-hk': ['zho-HK', 'yue-HK', 'zho-Hant', 'cmn-Hant']
  };
  const candidates = [
    ...(languageMap[normalized] || []),
    String(lang || '').trim(),
    normalized
  ].filter(Boolean);
  return [...new Set(candidates)];
}

function dedupeSubtitleEntries(entries) {
  const seen = new Set();
  return entries.filter(entry => {
    const key = `${entry.chapterIndex || ''}|${entry.url}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function extractSubtitleURLs(value, seen = new Set()) {
  const urls = [];
  if (!value || seen.has(value)) return urls;
  if (typeof value === 'object') seen.add(value);
  if (typeof value === 'string') {
    const text = value.trim();
    if (isPlayableSubtitleURL(text)) {
      return [text];
    }
    return urls;
  }
  if (Array.isArray(value)) {
    return value.flatMap(item => extractSubtitleURLs(item, seen));
  }
  if (typeof value === 'object') {
    for (const item of Object.values(value)) {
      urls.push(...extractSubtitleURLs(item, seen));
    }
  }
  return [...new Set(urls)];
}

function isPlayableSubtitleURL(text) {
  if (!/^https?:\/\//i.test(text)) return false;
  if (/\.terms\.vtt(?:[?#].*)?$/i.test(text)) return false;
  return /^https?:\/\/.+\.vtt(?:[?#].*)?$/i.test(text) || /^https?:\/\/.+\/subtitle\/.+/i.test(text);
}

function normalizeLang(lang) {
  return String(lang || '').trim().toLowerCase().replace('_', '-');
}

function writeSummary(summary) {
  fs.writeFileSync(SUMMARY_FILE, JSON.stringify(summary, null, 2), 'utf8');
}

function logExternalCollectionProgress(progress, totalDramas, externalDramas, nextPercent) {
  if (!externalDramas || nextPercent > 100) return nextPercent;
  const percent = Math.floor((progress.preparedDramas / externalDramas) * 100);
  if (percent < nextPercent) return nextPercent;
  const target = Math.min(100, Math.max(nextPercent, Math.floor(percent / 10) * 10));
  console.log(`📊 外挂剧收集进度 ${target}%：已收集 ${progress.preparedDramas}/${externalDramas} 部外挂剧（在线剧集总数 ${totalDramas} 部），字幕文件 ${progress.totalSubtitleUrls} 个`);
  return target + 10;
}

async function fetchWithRetry(url, options, label) {
  let lastError;
  for (let attempt = 1; attempt <= FETCH_RETRY_ATTEMPTS; attempt += 1) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    try {
      const response = await fetch(url, { ...options, signal: controller.signal });
      clearTimeout(timer);
      if (response.ok || attempt >= FETCH_RETRY_ATTEMPTS || !shouldRetryStatus(response.status)) {
        return response;
      }
      console.warn(`⚠️ ${label} HTTP ${response.status}，第 ${attempt}/${FETCH_RETRY_ATTEMPTS} 次失败，准备重试`);
    } catch (error) {
      clearTimeout(timer);
      lastError = error;
      if (attempt >= FETCH_RETRY_ATTEMPTS) {
        throw error;
      }
      console.warn(`⚠️ ${label} 请求异常，第 ${attempt}/${FETCH_RETRY_ATTEMPTS} 次失败，准备重试: ${error?.message || error}`);
    }
    await sleep(400 * attempt);
  }
  throw lastError || new Error(`${label} 请求失败`);
}

function shouldRetryStatus(status) {
  return status === 408 || status === 429 || status >= 500;
}

async function runPool(items, concurrency, worker) {
  let cursor = 0;
  const runners = Array.from({ length: concurrency }, async () => {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;
      await worker(items[index], index);
    }
  });
  await Promise.all(runners);
}

function closeWriter(writer) {
  return new Promise((resolve, reject) => {
    writer.end(error => {
      if (error) reject(error);
      else resolve();
    });
  });
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function clamp(value, min, max) {
  if (!Number.isFinite(value)) return min;
  return Math.max(min, Math.min(max, value));
}

main().catch(error => {
  console.error('🔥 剧集外挂字幕测试准备失败:', error);
  process.exit(1);
});
