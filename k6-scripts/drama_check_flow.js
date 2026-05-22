import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";

// ---------- 1. 自定义统计指标 ----------
const continuityErrors = new Counter('continuity_fail_count');
const offlineErrors = new Counter('offline_chapter_count');
const totalMismatchErrors = new Counter('total_mismatch_count');
const fetchErrors = new Counter('fetch_fail_count');
const updateStatusErrors = new Counter('update_status_fail_count');
const transportRetryCount = new Counter('transport_retry_count');
const retryCandidateCount = new Counter('retry_candidate_count');

// ---------- 2. 加载配置与数据 (Config & Drama Info) ----------
const k6Config = JSON.parse(open('./k6_config.json'));
const RETRY_MARKER_PREFIX = '__DRAMA_RETRY_720_NETWORK__|';
const PROGRESS_MARKER_PREFIX = '__DRAMA_CASE_DONE__|';
const isRetryPhase = __ENV.DRAMA_RETRY_PHASE === '1';
const retryIds = parseRetryIds(__ENV.DRAMA_RETRY_IDS || '');
const reportOutputDir = (__ENV.DRAMA_REPORT_DIR || 'report/api_report/.runtime/default').replace(/\/+$/, '');

const data = new SharedArray('drama_info_loader', function () {
    // 强制读取根目录下的 drama_info.json 文件 (由 Node 脚本生成)
    const fileContent = open('../drama_info.json');
    return [JSON.parse(fileContent)];
});

const config = data[0];
const DRAMA_LIST = retryIds.length > 0 ? retryIds : (config.dramaList || []);
const TOKEN = config.auth.x_token;
const API_BASE = config.apiBase || "http://35.225.224.94:8080";

// ---------- 3. 动态负载逻辑 ----------
const totalIds = DRAMA_LIST.length;
const vus = isRetryPhase
    ? Math.max(1, Math.min(k6Config.retryVus || 5, totalIds))
    : (k6Config.vus || Math.min(30, totalIds));
const itersPerVu = k6Config.itersPerVu || Math.ceil(totalIds / vus);

export const options = {
    scenarios: {
        drama_detection: {
            executor: 'per-vu-iterations',
            vus: vus,
            iterations: itersPerVu,
            maxDuration: k6Config.maxDuration || '10m',
        },
    },
    thresholds: {
        'continuity_fail_count': ['count>=0'],
        'offline_chapter_count': ['count>=0'],
        'total_mismatch_count': ['count>=0'],
        'update_status_fail_count': ['count>=0'],
    }
};

function parseRetryIds(rawValue) {
    if (!rawValue) return [];
    try {
        const parsed = JSON.parse(rawValue);
        if (Array.isArray(parsed)) {
            return parsed.map(String).filter(Boolean);
        }
    } catch (e) {
        // Fall back to comma-separated input for manual runs.
    }
    return rawValue.split(',').map(item => item.trim()).filter(Boolean);
}

// ---------- 4. 核心获取函数 ----------
function fetchDramaData(dramaId, resolution) {
    const url = `${API_BASE}/api/management/drama/chapter/list?drama_id=${dramaId}&page=1&page_size=200&resolution=${resolution}`;
    const params = {
        headers: {
            'Cookie': `x-token=${TOKEN}`,
            'Connection': 'keep-alive'
        },
        timeout: k6Config.timeout || '15s'
    };

    const maxAttempts = k6Config.retryAttempts || 3;
    let lastError = '';

    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        const res = http.get(url, params);
        if (res.status !== 200) {
            lastError = formatRequestError(res, attempt, maxAttempts);
            if (isTransportFailure(res.status) && attempt < maxAttempts) {
                transportRetryCount.add(1);
                sleep(0.2 * attempt);
                continue;
            }
            return { success: false, msg: lastError };
        }

        try {
            const body = JSON.parse(res.body);
            return { 
                success: true, 
                total: body.total || 0, 
                items: body.data || [] 
            };
        } catch (e) {
            return { success: false, msg: 'JSON 解析失败' };
        }
    }

    return { success: false, msg: lastError || '请求失败' };
}

function isTransportFailure(status) {
    return status === 0 || status >= 500;
}

function formatRequestError(res, attempt, maxAttempts) {
    const parts = [`HTTP ${res.status}`];
    if (res.error) parts.push(res.error);
    if (res.error_code) parts.push(`code=${res.error_code}`);
    parts.push(`attempt=${attempt}/${maxAttempts}`);
    return parts.join(' | ');
}

function markDramaCaseDone(dramaId) {
    console.log(`${PROGRESS_MARKER_PREFIX}${dramaId}`);
}

// ---------- 5. 数据校验逻辑 ----------
function analyzeHealth(items) {
    const errors = [];
    const chapterIndexes = items.map(c => c.index);

    if (chapterIndexes.length > 0) {
        const sortedIndex = [...chapterIndexes].sort((a, b) => a - b);
        const min = sortedIndex[0];
        const max = sortedIndex[sortedIndex.length - 1];
        let missing = [];
        for (let i = min; i <= max; i++) {
            if (!chapterIndexes.includes(i)) missing.push(i);
        }
        if (missing.length > 0) errors.push({ type: 'continuity', msg: `[跳号] 缺失Index: ${missing.join(', ')}`, count: 1 });
    }

    const offlineItems = items.filter(c => c.online == 0).map(c => c.index);
    if (offlineItems.length > 0) errors.push({ type: 'offline', msg: `[下架] Index: ${offlineItems.join(', ')}`, count: offlineItems.length });

    const failedConversionItems = items.filter(c => c.update_status == 0).map(c => c.index);
    if (failedConversionItems.length > 0) errors.push({ type: 'conversion', msg: `[转换失败] 异常Index: ${failedConversionItems.join(', ')}`, count: failedConversionItems.length });

    return { healthy: errors.length === 0, errors: errors };
}

function buildDramaSummaryLabel(dramaId, errors, fallback = false) {
    if (errors.length === 1) {
        const suffix = fallback ? ' (Fallback)' : '';
        return `剧集 ID: ${dramaId} - ${errors[0].msg}${suffix}`;
    }

    const fallbackSuffix = fallback ? ' - Fallback' : '';
    return `剧集 ID: ${dramaId} (发现 ${errors.length} 项异常${fallbackSuffix})`;
}

// ---------- 6. 主执行逻辑 ----------
export default function () {
    const index = (__VU - 1) * itersPerVu + __ITER;
    if (index >= totalIds) return;
    const dramaId = DRAMA_LIST[index];
    if (!dramaId) return;

    const res720 = fetchDramaData(dramaId, '720p');
    if (!res720.success) {
        if (!isRetryPhase) {
            const retryMarker = `${RETRY_MARKER_PREFIX}${dramaId}|${encodeURIComponent(res720.msg)}`;
            check(null, { [retryMarker]: false });
            retryCandidateCount.add(1);
            console.warn(`[↻] 剧集 ${dramaId} 720p 第一轮网络请求失败，加入二次尝试队列: ${res720.msg}`);
            markDramaCaseDone(dramaId);
            return;
        }

        const errorMsg = `<details style="cursor: pointer; color: #e74c3c;"><summary><b>剧集 ID: ${dramaId} (720p 网络请求失败)</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">错误原因: ${res720.msg}</div></details>`;
        console.error(`[🔥] 剧集 ${dramaId} 720p 二次尝试仍网络请求失败: ${res720.msg}`);
        check(null, { [errorMsg]: false });
        fetchErrors.add(1);
        markDramaCaseDone(dramaId);
        return;
    }

    const health720 = analyzeHealth(res720.items);
    if (health720.healthy && res720.total === res720.items.length) {
        sleep(0.1);
        markDramaCaseDone(dramaId);
        return; 
    }

    const res540 = fetchDramaData(dramaId, '540p');
    if (!res540.success) {
        reportFaults(dramaId, health720.errors, res720.total, res720.items.length);
        markDramaCaseDone(dramaId);
        return;
    }

    const items720 = res720.items;
    const items540 = res540.items;
    const allIndexes = Array.from(new Set([...items720.map(c => c.index), ...items540.map(c => c.index)]));
    const healthyUnion = [];
    const missingHealthy = [];

    allIndexes.sort((a, b) => a - b).forEach(idx => {
        const char720 = items720.find(c => c.index === idx);
        const char540 = items540.find(c => c.index === idx);
        const isOk720 = char720 && char720.online == 1 && char720.update_status == 1;
        const isOk540 = char540 && char540.online == 1 && char540.update_status == 1;

        if (isOk720 || isOk540) {
            healthyUnion.push(idx);
        } else {
            missingHealthy.push(idx);
        }
    });

    const unionErrors = [];
    if (healthyUnion.length > 0) {
        const min = healthyUnion[0];
        const max = healthyUnion[healthyUnion.length - 1];
        let missing = [];
        for (let i = min; i <= max; i++) {
            if (!healthyUnion.includes(i)) missing.push(i);
        }
        if (missing.length > 0) {
            unionErrors.push({ type: 'continuity', msg: `[跳号] 连续性缺失Index: ${missing.join(', ')} (双分辨率合力仍无解)`, count: 1 });
        }
    }

    if (missingHealthy.length > 0) {
        unionErrors.push({ type: 'unhealthy', msg: `[死锁异常] 这些章节在720p/540p均不健康: ${missingHealthy.join(', ')}`, count: missingHealthy.length });
    }

    const targetTotal = Math.max(res720.total, res540.total);
    if (healthyUnion.length !== targetTotal) {
        unionErrors.push({ type: 'mismatch', msg: `[计数] 并集健康集数(${healthyUnion.length}) 与标称总条目(${targetTotal}) 对不上`, count: 1 });
    }

    if (unionErrors.length > 0) {
        const errorDetails = [];
        unionErrors.forEach(err => {
            errorDetails.push(`• ${err.msg}`);
            if (err.type === 'continuity') continuityErrors.add(1);
            if (err.type === 'unhealthy') offlineErrors.add(err.count);
            if (err.type === 'mismatch') totalMismatchErrors.add(1);
        });

        const summaryLabel = buildDramaSummaryLabel(dramaId, unionErrors);
        const detailLines = errorDetails.join('<br>');
        const expandableMsg = `<details style="cursor: pointer; color: #e74c3c;"><summary><b>${summaryLabel}</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">${detailLines}</div></details>`;
        console.warn(`[❌ 实锤故障] ${summaryLabel}\n  ${errorDetails.join('\n  ')}`);
        check(null, { [expandableMsg]: false });
    }
    markDramaCaseDone(dramaId);
    sleep(0.1);
}

function reportFaults(dramaId, errors, total, actualLen) {
    if (total !== actualLen) {
        errors.push({ type: 'mismatch', msg: `[计数] Total(${total}) 与返回长度(${actualLen}) 不符`, count: 1 });
    }
    const errorDetails = [];
    errors.forEach(err => {
        errorDetails.push(`• ${err.msg}`);
        if (err.type === 'continuity') continuityErrors.add(1);
        if (err.type === 'offline') offlineErrors.add(err.count);
        if (err.type === 'conversion') updateStatusErrors.add(err.count);
        if (err.type === 'mismatch') totalMismatchErrors.add(1);
    });
    const summaryLabel = buildDramaSummaryLabel(dramaId, errors, true);
    const detailLines = errorDetails.join('<br>');
    const expandableMsg = `<details style="cursor: pointer; color: #e74c3c;"><summary><b>${summaryLabel}</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">${detailLines}</div></details>`;
    console.warn(`[❌ 实锤故障] ${summaryLabel}\n  ${errorDetails.join('\n  ')}`);
    check(null, { [expandableMsg]: false });
}

export function handleSummary(data) {
    const retryCandidates = collectRetryCandidates(data);
    const failedChecks = collectFailedChecks(data, name => !name.startsWith(RETRY_MARKER_PREFIX));
    stripRetryCandidateChecks(data, retryCandidates.length);

    const summaryText = `[📊] 执行完毕。汇总结果: 
        抓取失败数: ${data.metrics.fetch_fail_count ? data.metrics.fetch_fail_count.values.count : 0}
        720p网络失败待复验数: ${data.metrics.retry_candidate_count ? data.metrics.retry_candidate_count.values.count : 0}
        跳号剧集数: ${data.metrics.continuity_fail_count ? data.metrics.continuity_fail_count.values.count : 0}
        下架/异常章节总数: ${data.metrics.offline_chapter_count ? data.metrics.offline_chapter_count.values.count : 0}
        计数不符剧集数: ${data.metrics.total_mismatch_count ? data.metrics.total_mismatch_count.values.count : 0}`;

    console.log(summaryText);

    const nameMap = {
        'continuity_fail_count': 'Continuity Failures (跳号剧集数)',
        'offline_chapter_count': 'Unhealthy Chapters (下架/异常章节总数)',
        'total_mismatch_count': 'Total Mismatch (计数不符剧集数)',
        'fetch_fail_count': 'Fetch Failures (接口抓取失败数)',
        'transport_retry_count': 'Transport Retries (网络重试次数)',
        'retry_candidate_count': 'Retry Candidates (720p网络失败待复验数)',
    };

    const finalMetrics = {};
    Object.keys(data.metrics).forEach(key => {
        const translatedName = nameMap[key] || key;
        finalMetrics[translatedName] = data.metrics[key];
    });
    data.metrics = finalMetrics;

    const reportHtml = htmlReport(data, { title: k6Config.reportTitle || "Drama Detection Report (核心剧集章节检测报表)" });
    const unescapedHtml = reportHtml
        .replace(/&lt;details/g, '<details')
        .replace(/&lt;\/details&gt;/g, '</details>')
        .replace(/&lt;summary/g, '<summary')
        .replace(/&lt;\/summary&gt;/g, '</summary>')
        .replace(/&lt;br&gt;/g, '<br>')
        .replace(/&lt;b&gt;/g, '<b>')
        .replace(/&lt;\/b&gt;/g, '</b>')
        .replace(/&lt;div/g, '<div')
        .replace(/&lt;\/div&gt;/g, '</div>');

    const output = {};
    if (isRetryPhase) {
        output[`${reportOutputDir}/drama_retry_report.html`] = unescapedHtml;
        output[`${reportOutputDir}/drama_retry_result.json`] = JSON.stringify({
            generatedAt: new Date().toISOString(),
            phase: 'retry',
            retryIds,
            persistentFailures: failedChecks,
        }, null, 2);
        return output;
    }

    output[`${reportOutputDir}/drama_check_report.html`] = unescapedHtml;
    output[`${reportOutputDir}/drama_retry_candidates.json`] = JSON.stringify({
        generatedAt: new Date().toISOString(),
        phase: 'initial',
        candidates: retryCandidates,
    }, null, 2);
    return output;
}

function collectRetryCandidates(data) {
    const candidates = [];
    const seen = {};
    walkChecks(data.root_group, checkItem => {
        const name = checkItem.name || '';
        if (!name.startsWith(RETRY_MARKER_PREFIX) || !checkItem.fails) return;
        const rest = name.substring(RETRY_MARKER_PREFIX.length);
        const parts = rest.split('|');
        const dramaId = parts[0];
        if (!dramaId || seen[dramaId]) return;
        seen[dramaId] = true;
        candidates.push({
            dramaId,
            reason: decodeURIComponent(parts.slice(1).join('|') || ''),
        });
    });
    return candidates;
}

function collectFailedChecks(data, shouldInclude) {
    const failures = [];
    walkChecks(data.root_group, checkItem => {
        const name = checkItem.name || '';
        if (!checkItem.fails || !shouldInclude(name)) return;
        failures.push({
            name,
            fails: checkItem.fails || 0,
            passes: checkItem.passes || 0,
        });
    });
    return failures;
}

function stripRetryCandidateChecks(data, removedFailCount) {
    stripRetryCandidateChecksFromGroup(data.root_group);
    const checksMetric = data.metrics && data.metrics.checks && data.metrics.checks.values;
    if (!checksMetric || !removedFailCount) return;

    if (typeof checksMetric.fails === 'number') {
        checksMetric.fails = Math.max(0, checksMetric.fails - removedFailCount);
    }
    if (typeof checksMetric.count === 'number') {
        checksMetric.count = Math.max(0, checksMetric.count - removedFailCount);
    }
    if (typeof checksMetric.passes === 'number' && typeof checksMetric.fails === 'number') {
        const total = checksMetric.passes + checksMetric.fails;
        checksMetric.rate = total > 0 ? checksMetric.passes / total : 1;
        checksMetric.value = checksMetric.rate;
    }
}

function stripRetryCandidateChecksFromGroup(group) {
    if (!group) return;
    if (Array.isArray(group.checks)) {
        group.checks = group.checks.filter(checkItem => !(checkItem.name || '').startsWith(RETRY_MARKER_PREFIX));
    }
    if (Array.isArray(group.groups)) {
        group.groups.forEach(stripRetryCandidateChecksFromGroup);
    }
}

function walkChecks(group, visitor) {
    if (!group) return;
    if (Array.isArray(group.checks)) {
        group.checks.forEach(visitor);
    }
    if (Array.isArray(group.groups)) {
        group.groups.forEach(child => walkChecks(child, visitor));
    };
}
