import fs from 'fs';
import path from 'path';
import readline from 'readline';

const REPORT_DIR = process.env.SUBTITLE_REPORT_DIR || path.join('report', 'api_report', '.runtime', 'subtitle_manual');
const REPORT_ROOT = path.resolve(import.meta.dirname, '..', REPORT_DIR);
const QUEUE_FILE = path.join(REPORT_ROOT, 'subtitle_queue.jsonl');
const SUMMARY_FILE = path.join(REPORT_ROOT, 'subtitle_summary.json');
const FAILURE_FILE = path.join(REPORT_ROOT, 'subtitle_failures.jsonl');
const CACHE_ROOT = path.resolve(import.meta.dirname, '..', process.env.SUBTITLE_CACHE_DIR || path.join('report', 'api_report', '.subtitle-cache'));
const CACHE_FILE = path.join(CACHE_ROOT, 'subtitle_check_cache.jsonl');
const CACHE_ENABLED = process.env.SUBTITLE_CACHE_ENABLED !== '0';
const FETCH_CONCURRENCY = clamp(Number(process.env.SUBTITLE_FETCH_CONCURRENCY || 16), 1, 24);
const FETCH_TIMEOUT_MS = Number(process.env.SUBTITLE_FETCH_TIMEOUT_MS || 15000);
const FETCH_RETRY_ATTEMPTS = Number(process.env.SUBTITLE_FETCH_RETRY_ATTEMPTS || 3);
const MAX_SUBTITLE_SECONDS = Number(process.env.SUBTITLE_MAX_SECONDS || 600);
const AI_ENABLED = process.env.SUBTITLE_AI_ENABLED === '1' && Boolean(process.env.SUBTITLE_AI_API_KEY);
const AI_CONCURRENCY = clamp(Number(process.env.SUBTITLE_AI_CONCURRENCY || 1), 1, 2);
const AI_TIMEOUT_MS = Number(process.env.SUBTITLE_AI_TIMEOUT_MS || 30000);

const summary = loadSummary();
const counters = {
  checkedFiles: 0,
  failedFiles: 0,
  failures: 0,
  languageFailures: 0,
  timestampFailures: 0,
  fetchFailures: 0,
  missingSubtitleFailures: 0,
  subtitleCountFailures: 0,
  totalCheckItems: 0,
  uniqueTasks: 0,
  cacheHits: 0,
  cacheEnabled: CACHE_ENABLED
};

async function main() {
  fs.writeFileSync(FAILURE_FILE, '', 'utf8');
  const queue = await readQueue();
  const tasks = groupQueueEntries(queue);
  const cache = CACHE_ENABLED ? loadCache() : new Map();
  const cachedTasks = tasks.filter(task => cache.has(task.key)).length;
  counters.totalCheckItems = queue.length;
  counters.uniqueTasks = tasks.length;
  counters.cacheHits = cachedTasks;
  console.log(`🔎 开始检查外挂字幕：${queue.length} 个检查项，去重后 ${tasks.length} 个任务，并发 ${FETCH_CONCURRENCY}，缓存命中 ${cachedTasks} 个，AI复核 ${AI_ENABLED ? '启用' : '未启用（默认关闭）'}`);

  let nextCheckProgressPercent = 10;
  let nextSummaryWriteAt = 100;
  let lastHeartbeatAt = Date.now();
  await runPool(tasks, FETCH_CONCURRENCY, async task => {
    const result = await resolveTaskResult(task, cache);
    counters.checkedFiles += task.entries.length;
    if (result.failures.length > 0) {
      counters.failedFiles += task.entries.length;
      for (const entry of task.entries) {
        for (const failureTemplate of result.failures) {
          appendFailure(hydrateFailure(entry, failureTemplate));
        }
      }
    }
    nextCheckProgressPercent = logCheckProgress(counters, queue.length, nextCheckProgressPercent);
    lastHeartbeatAt = logHeartbeatProgress(counters, queue.length, lastHeartbeatAt);
    if (counters.checkedFiles >= nextSummaryWriteAt || counters.checkedFiles === queue.length) {
      writeSummary('running');
      while (nextSummaryWriteAt <= counters.checkedFiles) {
        nextSummaryWriteAt += 100;
      }
    }
  });

  writeSummary(counters.failures > 0 ? 'failed' : 'passed');
  console.log(`✅ 字幕检查完成：检查 ${counters.checkedFiles} 个文件，异常 ${counters.failures} 条`);
}

async function resolveTaskResult(task, cache) {
  const cached = cache.get(task.key);
  if (cached) {
    return cached;
  }
  const failures = (await checkSubtitleEntry(task.representative)).map(toFailureTemplate);
  const result = {
    key: task.key,
    failures,
    checkedAt: new Date().toISOString()
  };
  if (CACHE_ENABLED) {
    saveCacheResult(result);
    cache.set(task.key, result);
  }
  return result;
}

function groupQueueEntries(queue) {
  const map = new Map();
  for (const entry of queue) {
    const key = cacheKeyForEntry(entry);
    if (!map.has(key)) {
      map.set(key, { key, representative: entry, entries: [] });
    }
    map.get(key).entries.push(entry);
  }
  return [...map.values()];
}

function cacheKeyForEntry(entry) {
  if (entry.subtitleCountMismatch) {
    return `count:v1:${entry.dramaId || entry.intId || entry.title}`;
  }
  if (entry.missingSubtitle || !String(entry.url || '').trim()) {
    return `missing:v2:${entry.dramaId || entry.intId || entry.title}`;
  }
  return `url:v3:max=${MAX_SUBTITLE_SECONDS}:ai=${AI_ENABLED ? '1' : '0'}:${String(entry.url).trim()}`;
}

function toFailureTemplate(failure) {
  return {
    category: failure.category,
    message: failure.message,
    cue: failure.cue || null
  };
}

function hydrateFailure(entry, template) {
  return buildFailure(entry, template.category, template.message, template.cue);
}

async function checkSubtitleEntry(entry) {
  if (entry.subtitleCountMismatch) {
    return [buildFailure(
      entry,
      'subtitle_count',
      `字幕文件数量与剧集集数不一致，剧集集数 ${entry.dramaChapterCount || 0}，字幕文件 ${entry.subtitleFileCount || 0}`,
      {
        expectedChapters: Number(entry.dramaChapterCount || 0),
        subtitleFileCount: Number(entry.subtitleFileCount || 0)
      }
    )];
  }

  if (entry.missingSubtitle || !String(entry.url || '').trim()) {
    return [buildFailure(entry, 'missing_subtitle', '外挂剧未在章节数据中解析到 VTT 字幕地址')];
  }

  let content = '';
  try {
    content = await fetchText(entry.url);
  } catch (error) {
    return [buildFailure(entry, 'fetch', `字幕文件拉取失败: ${formatError(error)}`)];
  }

  const cues = parseVTT(content);
  const failures = [];
  const timestampFailures = detectTimestampFailures(cues);
  for (const cue of timestampFailures) {
    failures.push(buildFailure(entry, 'timestamp', cue.reason, cue));
  }

  const languageResult = await detectLanguageWithOptionalAI(entry, cues);
  if (languageResult.failed) {
    failures.push(buildFailure(entry, 'language', languageResult.reason, {
      expectedLang: entry.lang,
      detectedLang: languageResult.detectedLang,
      confidence: languageResult.confidence,
      aiReviewed: languageResult.aiReviewed,
      evidence: languageResult.evidence || []
    }));
  }

  return failures;
}

async function fetchText(url) {
  let lastError;
  for (let attempt = 1; attempt <= FETCH_RETRY_ATTEMPTS; attempt += 1) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    try {
      const response = await fetch(url, { signal: controller.signal, headers: { Connection: 'close' } });
      clearTimeout(timer);
      if (!response.ok) {
        if (attempt < FETCH_RETRY_ATTEMPTS && shouldRetryStatus(response.status)) {
          await sleep(300 * attempt);
          continue;
        }
        throw new Error(`HTTP ${response.status}`);
      }
      return await response.text();
    } catch (error) {
      clearTimeout(timer);
      lastError = error;
      if (attempt >= FETCH_RETRY_ATTEMPTS) {
        throw error;
      }
      await sleep(300 * attempt);
    }
  }
  throw lastError || new Error('fetch failed');
}

function shouldRetryStatus(status) {
  return status === 408 || status === 429 || status >= 500;
}

function parseVTT(content) {
  const lines = String(content || '').replace(/^\uFEFF/, '').split(/\r?\n/);
  const cues = [];
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i].trim();
    const match = line.match(/^([0-9:.]+)\s+-->\s+([0-9:.]+)/);
    if (!match) continue;

    const textLines = [];
    let cursor = i + 1;
    while (cursor < lines.length && lines[cursor].trim() !== '') {
      textLines.push(lines[cursor]);
      cursor += 1;
    }
    const startSeconds = parseTimestamp(match[1]);
    const endSeconds = parseTimestamp(match[2]);
    cues.push({
      index: cues.length + 1,
      start: match[1],
      end: match[2],
      startSeconds,
      endSeconds,
      text: textLines.join('\n').trim()
    });
    i = cursor;
  }
  return cues;
}

function parseTimestamp(value) {
  const parts = String(value).split(':').map(Number);
  if (parts.some(Number.isNaN)) return NaN;
  if (parts.length === 3) return parts[0] * 3600 + parts[1] * 60 + parts[2];
  if (parts.length === 2) return parts[0] * 60 + parts[1];
  return NaN;
}

function detectTimestampFailures(cues) {
  const failures = [];
  let previousStart = -1;
  for (const cue of cues) {
    if (!Number.isFinite(cue.startSeconds) || !Number.isFinite(cue.endSeconds)) {
      failures.push({ ...cue, reason: '字幕时间戳格式无法解析' });
    } else if (cue.endSeconds < cue.startSeconds) {
      failures.push({ ...cue, reason: '字幕结束时间早于开始时间' });
    } else if (cue.startSeconds > MAX_SUBTITLE_SECONDS || cue.endSeconds > MAX_SUBTITLE_SECONDS) {
      failures.push({ ...cue, reason: `字幕出现时间超过 ${Math.round(MAX_SUBTITLE_SECONDS / 60)} 分钟` });
    } else if (previousStart > cue.startSeconds) {
      failures.push({ ...cue, reason: '字幕时间轴倒退' });
    }
    if (Number.isFinite(cue.startSeconds)) {
      previousStart = Math.max(previousStart, cue.startSeconds);
    }
  }
  return failures;
}

async function detectLanguageWithOptionalAI(entry, cues) {
  const expected = normalizeExpectedLang(entry.lang);
  if (!expected) {
    return { failed: false, detectedLang: '', confidence: 0, aiReviewed: false };
  }
  const sampleCues = pickLanguageSampleCues(cues);
  const sample = sampleCues.map(cue => cue.text).filter(Boolean).join('\n').slice(0, 4000);
  if (!sample) {
    return { failed: true, reason: '字幕文件无可识别文本', detectedLang: '', confidence: 0, aiReviewed: false };
  }
  const local = detectLanguageLocal(sample);
  if (isJapaneseExpectedWithTraditionalChinese(expected, sample)) {
    return { failed: false, detectedLang: 'zh-tw', confidence: 0.99, aiReviewed: false };
  }
  const localMatched = expected.includes(local.lang);
  if (localMatched && local.confidence >= 0.35) {
    return { failed: false, detectedLang: local.lang, confidence: local.confidence, aiReviewed: false };
  }
  if (AI_ENABLED) {
    const ai = await enqueueAIReview(expected[0], entry.rawLang || entry.lang, local, sampleCues);
    if (ai && !ai.isMismatch) {
      return { failed: false, detectedLang: ai.detectedLang || local.lang, confidence: ai.confidence || local.confidence, aiReviewed: true };
    }
    if (ai) {
      return {
        failed: true,
        reason: `字幕语种与剧集语种不匹配，期望 ${entry.rawLang || entry.lang}，AI复核判断 ${ai.detectedLang || 'unknown'}。${ai.reason || ''}`.trim(),
        detectedLang: ai.detectedLang,
        confidence: ai.confidence,
        aiReviewed: true,
        evidence: ai.evidence || []
      };
    }
  }
  if (!localMatched && local.confidence >= 0.45) {
    return {
      failed: true,
      reason: `字幕语种与剧集语种不匹配，期望 ${entry.rawLang || entry.lang}，本地判断 ${local.lang || 'unknown'}`,
      detectedLang: local.lang,
      confidence: local.confidence,
      aiReviewed: false
    };
  }
  return { failed: false, detectedLang: local.lang, confidence: local.confidence, aiReviewed: false };
}

function pickLanguageSampleCues(cues) {
  const meaningful = cues.filter(cue => stripTags(cue.text || '').trim().length >= 2);
  if (meaningful.length <= 30) return meaningful;
  const sample = [];
  const step = Math.max(1, Math.floor(meaningful.length / 30));
  for (let i = 0; i < meaningful.length && sample.length < 30; i += step) {
    sample.push(meaningful[i]);
  }
  return sample;
}

function normalizeExpectedLang(lang) {
  const text = String(lang || '').trim().toLowerCase();
  const aliases = {
    cn: ['zh'], zh: ['zh'], 'zh-cn': ['zh'], chinese: ['zh'],
    en: ['en'], english: ['en'],
    es: ['es'], spanish: ['es'],
    pt: ['pt'], portuguese: ['pt'],
    fr: ['fr'], french: ['fr'],
    de: ['de'], german: ['de'],
    id: ['id'], indonesian: ['id'],
    ar: ['ar'], arabic: ['ar'],
    ru: ['ru'], russian: ['ru'],
    ja: ['ja'], japanese: ['ja'],
    ko: ['ko'], korean: ['ko'],
    th: ['th'], thai: ['th'],
    vi: ['vi'], vietnamese: ['vi']
  };
  return aliases[text] || aliases[text.split('-')[0]] || (text ? [text.split('-')[0]] : []);
}

function detectLanguageLocal(text) {
  const clean = stripTags(text).toLowerCase();
  const total = Math.max(clean.replace(/\s/g, '').length, 1);
  const scripts = [
    ['zh', /[\u4e00-\u9fff]/g],
    ['ja', /[\u3040-\u30ff]/g],
    ['ko', /[\uac00-\ud7af]/g],
    ['ar', /[\u0600-\u06ff]/g],
    ['ru', /[\u0400-\u04ff]/g],
    ['th', /[\u0e00-\u0e7f]/g]
  ].map(([lang, regex]) => ({ lang, score: (clean.match(regex)?.length || 0) / total }));
  const scriptBest = scripts.sort((a, b) => b.score - a.score)[0];
  if (scriptBest.score > 0.2) return { lang: scriptBest.lang, confidence: Math.min(0.99, scriptBest.score * 2) };

  const stopwordScores = {
    es: scoreWords(clean, [' que ', ' de ', ' la ', ' el ', ' los ', ' una ', ' por ', ' para ']),
    pt: scoreWords(clean, [' que ', ' de ', ' para ', ' nao ', ' uma ', ' por ', ' voce ', ' com ']),
    fr: scoreWords(clean, [' que ', ' de ', ' les ', ' une ', ' pas ', ' pour ', ' vous ', ' avec ']),
    de: scoreWords(clean, [' und ', ' der ', ' die ', ' das ', ' nicht ', ' ich ', ' sie ', ' ist ']),
    id: scoreWords(clean, [' yang ', ' dan ', ' tidak ', ' untuk ', ' aku ', ' kamu ', ' dengan ', ' dari ']),
    en: scoreWords(clean, [' the ', ' and ', ' you ', ' that ', ' have ', ' for ', ' not ', ' with ']),
    it: scoreWords(clean, [' che ', ' non ', ' per ', ' una ', ' del ', ' sono ', ' con ', ' gli ']),
    vi: scoreWords(clean, [' khong ', ' nguoi ', ' cua ', ' toi ', ' ban ', ' trong ', ' mot ', ' voi '])
  };
  const best = Object.entries(stopwordScores).sort((a, b) => b[1] - a[1])[0] || ['', 0];
  return { lang: best[0], confidence: Math.min(0.95, best[1] / 6) };
}

function isJapaneseExpectedWithTraditionalChinese(expected, sample) {
  return expected.includes('ja') && containsTraditionalChinese(sample);
}

function containsTraditionalChinese(text) {
  return /[個們來國會時過還對為與開關後點無麼體學實產當發見現電車書買賣讓說話認識邊這裡嗎問題氣長應該歡歲壞聽讀寫東從頭條愛處聲響]/.test(stripTags(text));
}

function scoreWords(text, words) {
  return words.reduce((sum, word) => sum + countOccurrences(text, word), 0);
}

function countOccurrences(text, needle) {
  let count = 0;
  let index = text.indexOf(needle);
  while (index >= 0) {
    count += 1;
    index = text.indexOf(needle, index + needle.length);
  }
  return count;
}

function stripTags(text) {
  return String(text || '').replace(/<[^>]+>/g, ' ').replace(/[^\p{L}\p{N}\s']/gu, ' ');
}

const aiQueue = [];
let activeAI = 0;

function enqueueAIReview(expectedLang, rawExpectedLang, local, sampleCues) {
  return new Promise(resolve => {
    aiQueue.push({ expectedLang, rawExpectedLang, local, sampleCues, resolve });
    drainAIQueue();
  });
}

function drainAIQueue() {
  while (activeAI < AI_CONCURRENCY && aiQueue.length > 0) {
    const item = aiQueue.shift();
    activeAI += 1;
    reviewLanguageWithAI(item.expectedLang, item.rawExpectedLang, item.local, item.sampleCues)
      .then(item.resolve)
      .catch(() => item.resolve(null))
      .finally(() => {
        activeAI -= 1;
        drainAIQueue();
      });
  }
}

async function reviewLanguageWithAI(expectedLang, rawExpectedLang, local, sampleCues) {
  const baseURL = String(process.env.SUBTITLE_AI_BASE_URL || 'https://api.deepseek.com').replace(/\/+$/, '');
  const model = process.env.SUBTITLE_AI_MODEL || 'deepseek-chat';
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), AI_TIMEOUT_MS);
  const sample = sampleCues.map(cue => `[${cue.index}] ${cue.start || '-'} --> ${cue.end || '-'} ${String(cue.text || '').replace(/\s+/g, ' ').trim()}`).join('\n').slice(0, 3500);
  const response = await fetch(`${baseURL}/chat/completions`, {
    method: 'POST',
    signal: controller.signal,
    headers: {
      Authorization: `Bearer ${process.env.SUBTITLE_AI_API_KEY}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model,
      temperature: 0,
      messages: [
        { role: 'system', content: '你是字幕语种复核器。只返回紧凑 JSON，不要 Markdown。JSON 格式：{"isMismatch":true,"detectedLang":"zh","confidence":0.95,"reason":"原因","evidence":[{"index":1,"text":"原字幕片段","reason":"为什么不符合"}]}。detectedLang 使用 ISO 639-1。只有确认字幕主要语种与期望不一致时 isMismatch 才为 true。' },
        { role: 'user', content: `期望剧集语种: ${rawExpectedLang || expectedLang} (${expectedLang})\n本地粗判: ${local.lang || 'unknown'} / ${local.confidence || 0}\n字幕样本:\n${sample}` }
      ]
    })
  }).finally(() => clearTimeout(timer));
  if (!response.ok) return null;
  const payload = await response.json();
  const text = payload?.choices?.[0]?.message?.content || '';
  const match = text.match(/\{[\s\S]*\}/);
  if (!match) return null;
  const parsed = JSON.parse(match[0]);
  const evidence = Array.isArray(parsed.evidence) ? parsed.evidence.slice(0, 5).map(item => ({
    index: Number(item?.index || 0),
    text: String(item?.text || '').slice(0, 500),
    reason: String(item?.reason || '').slice(0, 300)
  })).filter(item => item.text || item.reason) : [];
  return {
    isMismatch: Boolean(parsed.isMismatch),
    detectedLang: String(parsed.detectedLang || parsed.lang || '').toLowerCase().slice(0, 8),
    confidence: Number(parsed.confidence || 0),
    reason: String(parsed.reason || '').slice(0, 500),
    evidence
  };
}

function buildFailure(entry, category, message, cue = null) {
  const failure = {
    category,
    message,
    dramaId: entry.dramaId,
    intId: entry.intId,
    title: entry.title,
    cnName: entry.cnName,
    lang: entry.rawLang || entry.lang,
    chapterId: entry.chapterId,
    chapterIndex: entry.chapterIndex,
    dramaChapterCount: entry.dramaChapterCount,
    subtitleFileCount: entry.subtitleFileCount,
    url: entry.url,
    cue
  };
  return failure;
}

function appendFailure(failure) {
  counters.failures += 1;
  if (failure.category === 'language') counters.languageFailures += 1;
  if (failure.category === 'timestamp') counters.timestampFailures += 1;
  if (failure.category === 'fetch') counters.fetchFailures += 1;
  if (failure.category === 'missing_subtitle') counters.missingSubtitleFailures += 1;
  if (failure.category === 'subtitle_count') counters.subtitleCountFailures += 1;
  fs.appendFileSync(FAILURE_FILE, JSON.stringify(failure) + '\n', 'utf8');
}

async function readQueue() {
  if (!fs.existsSync(QUEUE_FILE)) return [];
  const stream = fs.createReadStream(QUEUE_FILE, 'utf8');
  const rl = readline.createInterface({ input: stream, crlfDelay: Infinity });
  const rows = [];
  for await (const line of rl) {
    if (!line.trim()) continue;
    rows.push(JSON.parse(line));
  }
  return rows;
}

function loadCache() {
  const cache = new Map();
  if (!fs.existsSync(CACHE_FILE)) {
    return cache;
  }
  const content = fs.readFileSync(CACHE_FILE, 'utf8');
  for (const line of content.split('\n')) {
    if (!line.trim()) continue;
    try {
      const item = JSON.parse(line);
      if (item?.key && Array.isArray(item.failures)) {
        cache.set(item.key, {
          key: item.key,
          failures: item.failures.map(toFailureTemplate),
          checkedAt: item.checkedAt || ''
        });
      }
    } catch {
      // Ignore broken cache rows. The next run will refresh that key.
    }
  }
  return cache;
}

function saveCacheResult(result) {
  fs.mkdirSync(CACHE_ROOT, { recursive: true });
  fs.appendFileSync(CACHE_FILE, JSON.stringify({
    key: result.key,
    failures: result.failures,
    checkedAt: result.checkedAt
  }) + '\n', 'utf8');
}

async function runPool(items, concurrency, worker) {
  let cursor = 0;
  const runners = Array.from({ length: concurrency }, async () => {
    while (cursor < items.length) {
      const item = items[cursor];
      cursor += 1;
      await worker(item);
    }
  });
  await Promise.all(runners);
}

function loadSummary() {
  try {
    return JSON.parse(fs.readFileSync(SUMMARY_FILE, 'utf8'));
  } catch {
    return {};
  }
}

function writeSummary(status) {
  fs.writeFileSync(SUMMARY_FILE, JSON.stringify({
    ...summary,
    ...counters,
    aiEnabled: AI_ENABLED,
    status,
    finishedAt: new Date().toISOString()
  }, null, 2), 'utf8');
}

function logCheckProgress(current, total, nextPercent) {
  if (!total || nextPercent > 100) return nextPercent;
  const percent = Math.floor((current.checkedFiles / total) * 100);
  let target = nextPercent;
  while (percent >= target && target <= 100) {
    console.log(`📊 字幕检查进度 ${target}%：已检查 ${current.checkedFiles}/${total} 个检查项，异常 ${current.failures} 条`);
    target += 10;
  }
  return target;
}

function logHeartbeatProgress(current, total, lastLoggedAt) {
  const now = Date.now();
  if (now - lastLoggedAt < 60_000 || current.checkedFiles >= total) {
    return lastLoggedAt;
  }
  const percent = total ? ((current.checkedFiles / total) * 100).toFixed(2) : '0.00';
  console.log(`⏱️ 字幕检查心跳：已检查 ${current.checkedFiles}/${total} (${percent}%)，异常 ${current.failures} 条，缓存命中 ${current.cacheHits} 个`);
  return now;
}

function clamp(value, min, max) {
  if (!Number.isFinite(value)) return min;
  return Math.max(min, Math.min(max, value));
}

function formatError(error) {
  return error?.message || String(error);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

main().catch(error => {
  writeSummary('error');
  console.error('🔥 剧集外挂字幕检查失败:', error);
  process.exit(1);
});
