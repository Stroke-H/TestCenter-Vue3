import http from 'k6/http';
import { BUSINESS_RULE_VERSION } from './drama_business_rules.js';
import { DRAMA_SCOPE_VERSION } from './drama_scope.js';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { htmlReport } from "./vendor/k6-reporter/bundle.js";

// ---------- 1. 自定义统计指标 ----------
const continuityErrors = new Counter('continuity_fail_count');
const offlineErrors = new Counter('offline_chapter_count');
const totalMismatchErrors = new Counter('total_mismatch_count');
const fetchErrors = new Counter('fetch_fail_count');
const updateStatusErrors = new Counter('update_status_fail_count');
const transportRetryCount = new Counter('transport_retry_count');
const retryCandidateCount = new Counter('retry_candidate_count');
const totalDramaEpisodeCount = new Counter('total_drama_episode_count');
const unlockTypeRuleErrors = new Counter('unlock_type_rule_fail_count');
const appGroupMetadataErrors = new Counter('app_group_metadata_fail_count');
const namingRuleErrors = new Counter('naming_rule_fail_count');
const classificationRuleErrors = new Counter('classification_rule_fail_count');
const freeChapterErrors = new Counter('free_chapters_fail_count');
const copyrightErrors = new Counter('copyright_fail_count');
const descriptionLanguageErrors = new Counter('description_language_fail_count');

// ---------- 2. 加载配置与数据 (Config & Drama Info) ----------
const k6Config = JSON.parse(open('./k6_config.json'));
// Temporary business switches, snapshotted by each run; missing means enabled.
const skipGroupChecks = k6Config.skipGroupChecks === true;
const skipUnlockTypeChecks = k6Config.skipUnlockTypeChecks === true;
const skipFreeChapterChecks = k6Config.skipFreeChapterChecks === true;
const skipCopyrightChecks = k6Config.skipCopyrightChecks === true;
const skipDubbingClassificationChecks = k6Config.skipDubbingClassificationChecks === true;
const retryPolicy = {direct:['group','unlock_type','free_chapters','copyright'],confirm:['app_group_metadata','dubbing_classification','description_language','720_fetch']};
const RETRY_MARKER_PREFIX = '__DRAMA_RETRY_CANDIDATE__|';
const LEGACY_RETRY_MARKER_PREFIX = '__DRAMA_RETRY_720_NETWORK__|';
const PROGRESS_MARKER_PREFIX = '__DRAMA_CASE_DONE__|';
const isRetryPhase = __ENV.DRAMA_RETRY_PHASE === '1';
const selectedIds = __ENV.DRAMA_CASE_IDS_FILE ? JSON.parse(open(__ENV.DRAMA_CASE_IDS_FILE)) : null;
const retryIds = selectedIds || parseRetryIds(__ENV.DRAMA_RETRY_IDS || '');
const metadataRetryIds = new Set(__ENV.DRAMA_METADATA_RETRY_IDS_FILE ? JSON.parse(open(__ENV.DRAMA_METADATA_RETRY_IDS_FILE)) : []);
const reportOutputDir = (__ENV.DRAMA_REPORT_DIR || 'report/api_report/.runtime/default').replace(/\/+$/, '');

const data = new SharedArray('drama_case_records', function () {
    // Parse once in SharedArray's initializer. Each VU only copies the small
    // header and its current record, never a single element containing all meta.
    const prepared = JSON.parse(open('../drama_info.json'));
    const included = new Set((prepared.inspectionScope || {}).includedIds || []);
    if ((prepared.dramaList || []).some(id => !included.has(id)) || included.size !== (prepared.dramaList || []).length) {
        throw new Error('剧集名单与已确认的白名单范围不一致，请重新准备数据');
    }
    const header = {
        auth: prepared.auth, apiBase: prepared.apiBase, dramaList: prepared.dramaList,
        descriptionLanguageValidation: prepared.descriptionLanguageValidation,
        preparedRuleValidation: prepared.preparedRuleValidation,
        inspectionScope: prepared.inspectionScope && {
            version: prepared.inspectionScope.version,
            sourceCount: prepared.inspectionScope.sourceCount,
            excludedIds: prepared.inspectionScope.excludedIds,
        },
    };
    return [header, ...(prepared.dramaList || []).map(id => {
        const meta = (prepared.dramaMeta || {})[id];
        if (!meta || !meta.prepared_rules) throw new Error(`剧集 ${id} 缺少收集阶段的字段规则结果，请重新准备数据`);
        for (const key of ['directErrors', 'metadataErrors', 'classificationErrors', 'resolvedGroupErrors']) {
            if (!Array.isArray(meta.prepared_rules[key])) throw new Error(`剧集 ${id} 的字段规则结果不完整，请重新准备数据`);
        }
        const language = meta.description_language_check;
        if (!language || !language.status) throw new Error(`剧集 ${id} 缺少简介语种检测结果，请重新准备数据`);
        const needsDescription = ['mismatch', 'invalid_lang', 'uncertain', 'mixed', 'unsupported'].includes(language.status);
        return {
            id, int_id: meta.int_id, title: meta.title, cn_title: meta.cn_title, cn_name: meta.cn_name,
            lang: meta.lang, description: needsDescription ? meta.description : '',
            description_language_check: language, prepared_rules: meta.prepared_rules,
        };
    })];
});

const config = data[0];
if (!config.descriptionLanguageValidation || !config.descriptionLanguageValidation.ready) {
    throw new Error('简介语种检测数据未准备完成，请重启新版后端并重新运行完整剧集测试。');
}
const DRAMA_LIST = selectedIds || (retryIds.length > 0 ? retryIds : (config.dramaList || []));
const DRAMA_INDEX = new Map(config.dramaList.map((id, index) => [id, index + 1]));
if(config.inspectionScope?.version!==DRAMA_SCOPE_VERSION) throw new Error('白名单筛选数据未准备，请使用新版完整测试入口重新准备数据');
const preparedRules = config.preparedRuleValidation;
if (!preparedRules || !preparedRules.ready || preparedRules.version !== BUSINESS_RULE_VERSION || preparedRules.checkedCount !== config.dramaList.length || preparedRules.phase !== (isRetryPhase ? 'retry' : 'initial')) {
    throw new Error('收集阶段字段规则未完成或版本不一致，请使用完整测试入口重新准备数据');
}
for (const key of ['skipGroupChecks', 'skipUnlockTypeChecks', 'skipFreeChapterChecks', 'skipCopyrightChecks', 'skipDubbingClassificationChecks']) {
    if (!preparedRules.settings || preparedRules.settings[key] !== (k6Config[key] === true)) throw new Error('字段规则开关与收集快照不一致，请重新准备数据');
}
for(const id of DRAMA_LIST)if(!DRAMA_INDEX.has(id))throw new Error(`剧集${id}不在已确认的白名单范围，禁止继续检测`);
const TOKEN = config.auth.x_token;
const API_BASE = config.apiBase || "http://35.225.224.94:8080";

// ---------- 3. 动态负载逻辑 ----------
const totalIds = DRAMA_LIST.length;
const vus = isRetryPhase
    ? Math.max(1, Math.min(k6Config.retryVus || 5, totalIds))
    : Math.max(1,Math.min(k6Config.vus || 30,Math.max(totalIds,1)));
const itersPerVu = Math.max(1,k6Config.itersPerVu || Math.ceil(totalIds / vus));

export const options = {
    scenarios: {
        drama_detection: {
            executor: 'per-vu-iterations',
            vus: vus,
            iterations: itersPerVu,
            maxDuration: k6Config.maxDuration || '30m',
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
    // A successful bookkeeping check is aggregated across all VUs by k6.
    // VU-local arrays cannot provide a trustworthy completed-ID inventory.
    check(null, { [`__DRAMA_COMPLETED__|${dramaId}`]: true });
    console.log(`${PROGRESS_MARKER_PREFIX}${dramaId}`);
}

function queueSecondConfirmation(dramaId, kind, reasons) {
    const reasonText = (reasons || []).filter(Boolean).join('；');
    const retryMarker = `${RETRY_MARKER_PREFIX}${dramaId}|${kind}|${encodeURIComponent(reasonText)}`;
    check(null, { [retryMarker]: false });
    retryCandidateCount.add(1);
}

// ---------- 5. 数据校验逻辑 ----------
function getChapterIndex(item) {
    return Number(item && item.index);
}

function isValidChapterIndex(index) {
    return Number.isFinite(index);
}

function isChapterOnline(item) {
    return item && Number(item.online) === 1;
}

function isChapterErrored(item) {
    if (!item) return false;
    if (!isChapterOnline(item)) return true;
    return Number(item.update_status) > 1;
}

function isChapterConverted(item) {
    return item && Number(item.update_status) === 1;
}

function isHealthyChapter(item) {
    return isChapterOnline(item) && isChapterConverted(item) && !isChapterErrored(item);
}

function uniqueSortedIndexes(items) {
    return Array.from(new Set((items || [])
        .map(getChapterIndex)
        .filter(isValidChapterIndex)))
        .sort((a, b) => a - b);
}

function findMissingIndexes(indexes) {
    if (!indexes || indexes.length === 0) return [];
    const present = new Set(indexes);
    const min = indexes[0];
    const max = indexes[indexes.length - 1];
    const missing = [];
    for (let i = min; i <= max; i++) {
        if (!present.has(i)) missing.push(i);
    }
    return missing;
}

function buildItemMap(items) {
    const map = {};
    (items || []).forEach(item => {
        const index = getChapterIndex(item);
        if (isValidChapterIndex(index)) {
            map[index] = item;
        }
    });
    return map;
}

function analyzeHealth(items, options = {}) {
    const errors = [];
    const indexes = uniqueSortedIndexes(items);

    if (options.includeDirectJump) {
        const missing = findMissingIndexes(indexes);
        if (missing.length > 0) {
            errors.push({ type: 'continuity', msg: `[直接跳集] 两个相邻章节之间缺失Index: ${missing.join(', ')}`, count: 1 });
        }
    }

    const erroredItems = items
        .filter(isChapterErrored)
        .map(getChapterIndex)
        .filter(isValidChapterIndex);
    if (erroredItems.length > 0) {
        errors.push({ type: 'offline', msg: `[出错] 章节状态异常Index: ${erroredItems.join(', ')}`, count: erroredItems.length });
    }

    const convertingItems = items
        .filter(c => isChapterOnline(c) && !isChapterConverted(c) && !isChapterErrored(c))
        .map(getChapterIndex)
        .filter(isValidChapterIndex);
    if (convertingItems.length > 0) {
        errors.push({ type: 'conversion', msg: `[转换中] 章节尚未完成转换Index: ${convertingItems.join(', ')}`, count: convertingItems.length });
    }

    return { healthy: errors.length === 0, errors: errors };
}

function explainUnhealthyIndex(index, item720, item540) {
    const candidates = [item720, item540].filter(Boolean);
    if (candidates.some(isChapterErrored)) {
        return 'offline';
    }
    if (candidates.some(item => !isChapterConverted(item))) {
        return 'conversion';
    }
    return 'continuity';
}

function buildUnionErrors(items720, items540) {
    const map720 = buildItemMap(items720);
    const map540 = buildItemMap(items540);
    const allIndexes = uniqueSortedIndexes([...(items720 || []), ...(items540 || [])]);
    const missingIndexes = findMissingIndexes(allIndexes);
    const erroredIndexes = [];
    const convertingIndexes = [];

    allIndexes.forEach(index => {
        const item720 = map720[index];
        const item540 = map540[index];
        const isOk720 = item720 && isHealthyChapter(item720);
        const isOk540 = item540 && isHealthyChapter(item540);
        if (isOk720 || isOk540) return;

        const reason = explainUnhealthyIndex(index, item720, item540);
        if (reason === 'offline') {
            erroredIndexes.push(index);
        } else if (reason === 'conversion') {
            convertingIndexes.push(index);
        }
    });

    const errors = [];
    if (erroredIndexes.length > 0) {
        errors.push({ type: 'offline', msg: `[出错] 720p/540p 均无法提供健康章节Index: ${erroredIndexes.join(', ')}`, count: erroredIndexes.length });
    }
    if (convertingIndexes.length > 0) {
        errors.push({ type: 'conversion', msg: `[转换中] 720p/540p 均未完成转换Index: ${convertingIndexes.join(', ')}`, count: convertingIndexes.length });
    }
    if (missingIndexes.length > 0) {
        errors.push({ type: 'continuity', msg: `[直接跳集] 720p/540p 均不存在的Index: ${missingIndexes.join(', ')}`, count: 1 });
    }
    return errors;
}

function buildCountMismatchError(total, healthyCount, existingErrors) {
    if (healthyCount === total) return null;
    if (existingErrors && existingErrors.length > 0) return null;
    return { type: 'mismatch', msg: `[计数] 健康集数(${healthyCount}) 与标称总条目(${total}) 对不上`, count: 1 };
}

function getRawDramaMeta(dramaId) {
    const index = DRAMA_INDEX.get(dramaId);
    if (index === undefined) throw new Error(`剧集 ${dramaId} 不在本轮已收集的白名单中`);
    return data[index];
}

function getPreparedRuleResult(dramaId) {
    const result = getRawDramaMeta(dramaId).prepared_rules;
    for (const key of ['directErrors', 'metadataErrors', 'classificationErrors', 'resolvedGroupErrors']) {
        if (!result || !Array.isArray(result[key])) throw new Error(`剧集 ${dramaId} 的字段规则结果不完整，请重新准备数据`);
    }
    return result;
}

















function buildDramaSummaryLabel(dramaId, errors, fallback = false) {
    const identity = formatDramaIdentity(dramaId);
    const fallbackSuffix = fallback ? ' - Fallback' : '';
    return `${identity}${fallbackSuffix}`;
}

function formatDramaIdentity(dramaId) {
    const meta = getDramaMeta(dramaId);
    const titleLine = `${meta.title || '未知标题'}/CnName:${meta.cnTitle || '未知中文名'}`;
    const idLine = `ID:${dramaId}${meta.intId ? `(${meta.intId})` : ''}`;
    return `${titleLine}<br>${idLine}`;
}

function formatDramaIdentityPlain(dramaId) {
    const meta = getDramaMeta(dramaId);
    const titleLine = `${meta.title || '未知标题'}/CnName:${meta.cnTitle || '未知中文名'}`;
    const idLine = `ID:${dramaId}${meta.intId ? `(${meta.intId})` : ''}`;
    return `${titleLine}\n${idLine}`;
}

function getDramaMeta(dramaId) {
    const meta = getRawDramaMeta(dramaId);
    return {
        intId: String(meta.int_id || meta.intId || '').trim(),
        title: String(meta.title || meta.name || '').trim(),
        cnTitle: String(meta.cn_title || meta.cnTitle || meta.cn_name || meta.cnName || '').trim(),
        cnName: String(meta.cn_name || meta.cnName || meta.cn_title || meta.cnTitle || '').trim(),
    };
}



function getDescriptionLanguageIssue(dramaId) {
    const meta = getRawDramaMeta(dramaId);
    const result = meta.description_language_check;
    if (!result) throw new Error(`剧集 ${dramaId} 缺少简介语种检测结果`);
    if (result.status !== 'mismatch' && result.status !== 'invalid_lang') return '';
    const escapeText = value => String(value || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/[\r\n]+/g, ' ');
    const lang = escapeText(meta.lang) || '空';
    const detail = result.status === 'mismatch'
        ? `识别语种=${result.detected}，与 lang=${lang} 不一致（本地识别疑似异常，待人工复核）`
        : `lang=${lang} 为空或不在支持的14种语言内`;
    return `[简介语种] ${detail}。description=${escapeText(meta.description)}；内容指纹=${result.description_hash}`;
}

// ---------- 6. 主执行逻辑 ----------
export default function () {
    const index = (__VU - 1) * itersPerVu + __ITER;
    if (index >= totalIds) return;
    const dramaId = DRAMA_LIST[index];
    if (!dramaId) return;

    // 简介检查覆盖全部在线剧集，必须先于任何临时直接 PASS 判断。
    const descriptionLanguageIssue = getDescriptionLanguageIssue(dramaId);
    const descriptionRuleErrors = [];
    if (descriptionLanguageIssue) {
        if (isRetryPhase) {
            descriptionRuleErrors.push({ type: 'language', msg: descriptionLanguageIssue, count: 1 });
        } else {
            queueSecondConfirmation(dramaId, 'description_language', [descriptionLanguageIssue]);
        }
    }

    // Collection has already evaluated every metadata-only rule. k6 only
    // incorporates its findings; language comparison and playback stay here.
    const prepared = getPreparedRuleResult(dramaId);
    const directRuleErrors = isRetryPhase ? [] : prepared.directErrors;
    const metadataConfirmationErrors = isRetryPhase && metadataRetryIds.has(dramaId) && !prepared.metadataErrors.length
        ? prepared.resolvedGroupErrors : prepared.metadataErrors;
    const classificationErrors = prepared.classificationErrors;
    const deferredBusinessErrors=[...metadataConfirmationErrors,...classificationErrors];
    if(!isRetryPhase&&metadataConfirmationErrors.length)queueSecondConfirmation(dramaId,'app_group_metadata_case',metadataConfirmationErrors.map(error=>error.msg));
    if(!isRetryPhase&&classificationErrors.length)queueSecondConfirmation(dramaId,'dubbing_classification',classificationErrors.map(error=>error.msg));
    const validationRuleErrors=[...directRuleErrors,...descriptionRuleErrors,...(isRetryPhase?deferredBusinessErrors:[])];

    const res720 = fetchDramaData(dramaId, '720p');
    if (!res720.success) {
        if (!isRetryPhase) {
            queueSecondConfirmation(dramaId, '720_fetch', [res720.msg]);
            console.warn(`[↻] 剧集 ${dramaId} 720p 第一轮网络请求失败，加入二次尝试队列: ${res720.msg}`);
            if (validationRuleErrors.length > 0) {
                reportFaults(dramaId, validationRuleErrors, 0, 0);
            }
            markDramaCaseDone(dramaId);
            return;
        }

        console.error(`[🔥] 剧集 ${dramaId} 720p 二次尝试仍网络请求失败: ${res720.msg}`);
        reportFaults(dramaId, [
            { type: 'fetch', msg: `[接口抓取失败] 720p 二次确认仍失败。错误原因: ${res720.msg}`, count: 1 },
            ...validationRuleErrors,
        ], 0, 0);
        markDramaCaseDone(dramaId);
        return;
    }
    totalDramaEpisodeCount.add(res720.total || res720.items.length || 0);

    const health720 = analyzeHealth(res720.items, { includeDirectJump: true });
    if (health720.healthy && res720.total === res720.items.length) {
        if (validationRuleErrors.length > 0) {
            reportFaults(dramaId, validationRuleErrors, res720.total, res720.items.length);
        }
        sleep(0.1);
        markDramaCaseDone(dramaId);
        return; 
    }

    const res540 = fetchDramaData(dramaId, '540p');
    if (!res540.success) {
        reportFaults(dramaId, [...health720.errors, ...validationRuleErrors], res720.total, res720.items.length);
        markDramaCaseDone(dramaId);
        return;
    }

    const items720 = res720.items;
    const items540 = res540.items;
    const healthyUnion = uniqueSortedIndexes([...items720, ...items540].filter(isHealthyChapter));
    const unionErrors = buildUnionErrors(items720, items540);

    const targetTotal = Math.max(res720.total, res540.total);
    const countMismatch = buildCountMismatchError(targetTotal, healthyUnion.length, unionErrors);
    if (countMismatch) {
        unionErrors.push(countMismatch);
    }
    const finalErrors = [...unionErrors, ...validationRuleErrors];

    if (finalErrors.length > 0) {
        const errorDetails = [];
        finalErrors.forEach(err => {
            errorDetails.push(`• ${err.msg}`);
            if (err.type === 'continuity') continuityErrors.add(1);
            if (err.type === 'offline') offlineErrors.add(err.count);
            if (err.type === 'conversion') updateStatusErrors.add(err.count);
            if (err.type === 'mismatch') totalMismatchErrors.add(1);
            if (err.type === 'unlock') unlockTypeRuleErrors.add(err.count);
            if (err.type === 'metadata') appGroupMetadataErrors.add(err.count);
            if (err.type === 'naming') namingRuleErrors.add(err.count);
            if (err.type === 'classification') classificationRuleErrors.add(err.count);
            if (err.type === 'language') descriptionLanguageErrors.add(err.count);
            if (err.type === 'free_chapters') freeChapterErrors.add(err.count);
            if (err.type === 'copyright') copyrightErrors.add(err.count);
            if (err.type === 'fetch') fetchErrors.add(err.count);
        });

        const summaryLabel = buildDramaSummaryLabel(dramaId, finalErrors);
        const detailLines = errorDetails.join('<br>');
        const expandableMsg = `<details style="cursor: pointer;"><summary><b>${summaryLabel}</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">${detailLines}</div></details>`;
        console.warn(`[❌ 实锤故障] ${formatDramaIdentityPlain(dramaId)}\n  ${errorDetails.join('\n  ')}`);
        check(null, { [expandableMsg]: false });
    }
    markDramaCaseDone(dramaId);
    sleep(0.1);
}

function reportFaults(dramaId, errors, total, actualLen) {
    if (total !== actualLen && errors.length === 0) {
        errors.push({ type: 'mismatch', msg: `[计数] Total(${total}) 与返回长度(${actualLen}) 不符`, count: 1 });
    }
    const errorDetails = [];
    errors.forEach(err => {
        errorDetails.push(`• ${err.msg}`);
        if (err.type === 'continuity') continuityErrors.add(1);
        if (err.type === 'offline') offlineErrors.add(err.count);
        if (err.type === 'conversion') updateStatusErrors.add(err.count);
        if (err.type === 'mismatch') totalMismatchErrors.add(1);
        if (err.type === 'unlock') unlockTypeRuleErrors.add(err.count);
        if (err.type === 'metadata') appGroupMetadataErrors.add(err.count);
        if (err.type === 'naming') namingRuleErrors.add(err.count);
        if (err.type === 'classification') classificationRuleErrors.add(err.count);
        if (err.type === 'language') descriptionLanguageErrors.add(err.count);
        if (err.type === 'free_chapters') freeChapterErrors.add(err.count);
        if (err.type === 'copyright') copyrightErrors.add(err.count);
        if (err.type === 'fetch') fetchErrors.add(err.count);
    });
    const summaryLabel = buildDramaSummaryLabel(dramaId, errors, true);
    const detailLines = errorDetails.join('<br>');
    const expandableMsg = `<details style="cursor: pointer;"><summary><b>${summaryLabel}</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">${detailLines}</div></details>`;
    console.warn(`[❌ 实锤故障] ${formatDramaIdentityPlain(dramaId)}\n  ${errorDetails.join('\n  ')}`);
    check(null, { [expandableMsg]: false });
}

export function handleSummary(data) {
    const completedIds = [];
    let completionPasses = 0;
    walkChecks(data.root_group, item => {
        if (item.name.startsWith('__DRAMA_COMPLETED__|') && item.passes > 0) {
            completedIds.push(item.name.slice('__DRAMA_COMPLETED__|'.length));
            completionPasses += item.passes;
        }
    });
    stripCompletionChecks(data, completionPasses);
    const retryCandidates = collectRetryCandidates(data);
    const retryMarkerFailCount = countRetryCandidateChecks(data);
    const failedChecks = collectFailedChecks(data, name => !isRetryMarkerName(name));
    const structuredFailures = collectDramaFailureSummaries(data);
    const totalEpisodes = getMetricCount(data, 'total_drama_episode_count') || totalIds;
    stripRetryCandidateChecks(data, retryMarkerFailCount);

    const summaryText = `[📊] 执行完毕。汇总结果: 
        抓取失败数: ${data.metrics.fetch_fail_count ? data.metrics.fetch_fail_count.values.count : 0}
        二次确认候选异常数: ${data.metrics.retry_candidate_count ? data.metrics.retry_candidate_count.values.count : 0}
        直接跳集剧集数: ${data.metrics.continuity_fail_count ? data.metrics.continuity_fail_count.values.count : 0}
        出错章节总数: ${data.metrics.offline_chapter_count ? data.metrics.offline_chapter_count.values.count : 0}
        转换中章节总数: ${data.metrics.update_status_fail_count ? data.metrics.update_status_fail_count.values.count : 0}
        解锁类型规则异常剧集数: ${data.metrics.unlock_type_rule_fail_count ? data.metrics.unlock_type_rule_fail_count.values.count : 0}
        App Group元数据异常剧集数: ${data.metrics.app_group_metadata_fail_count ? data.metrics.app_group_metadata_fail_count.values.count : 0}
        命名规范异常数: ${data.metrics.naming_rule_fail_count ? data.metrics.naming_rule_fail_count.values.count : 0}
        分类规则异常剧集数: ${data.metrics.classification_rule_fail_count ? data.metrics.classification_rule_fail_count.values.count : 0}
        简介语种异常剧集数: ${data.metrics.description_language_fail_count ? data.metrics.description_language_fail_count.values.count : 0}
        检查剧集总量: ${totalEpisodes}（${totalIds}部剧）
        计数不符剧集数: ${data.metrics.total_mismatch_count ? data.metrics.total_mismatch_count.values.count : 0}`;

    console.log(summaryText);

    const nameMap = {
        'continuity_fail_count': 'Direct Skips (直接跳集剧集数)',
        'offline_chapter_count': 'Errored Chapters (出错章节总数)',
        'total_mismatch_count': 'Total Mismatch (计数不符剧集数)',
        'fetch_fail_count': 'Fetch Failures (接口抓取失败数)',
        'update_status_fail_count': 'Converting Chapters (转换中章节总数)',
        'transport_retry_count': 'Transport Retries (网络重试次数)',
        'retry_candidate_count': 'Retry Candidates (二次确认候选异常数)',
        'total_drama_episode_count': 'Total Drama Episodes (总剧集数)',
        'unlock_type_rule_fail_count': 'Unlock Type Rule Failures (解锁类型规则异常剧集数)',
        'app_group_metadata_fail_count': 'App Group Metadata Failures (App Group元数据异常剧集数)',
        'naming_rule_fail_count': 'Naming Rule Failures (命名规范异常数)',
        'classification_rule_fail_count': 'Classification Rule Failures (分类规则异常剧集数)',
        'free_chapters_fail_count': 'Free Chapter Failures (免费集数异常剧集数)',
        'copyright_fail_count': 'Copyright Failures (版权异常剧集数)',
        'description_language_fail_count': 'Description Language Failures (简介语种异常剧集数)',
    };

    const finalMetrics = {};
    Object.keys(data.metrics).forEach(key => {
        const translatedName = nameMap[key] || key;
        finalMetrics[translatedName] = data.metrics[key];
    });
    data.metrics = finalMetrics;

    const output = {};
    const completedSet = new Set(completedIds);
    output[`${reportOutputDir}/drama_${isRetryPhase ? 'retry' : 'initial'}_completion.json`] = JSON.stringify({
        phase: isRetryPhase ? 'retry' : 'initial',
        expectedIds: DRAMA_LIST,
        completedIds: [...completedSet],
        missingIds: DRAMA_LIST.filter(id => !completedSet.has(id)),
    }, null, 2);
    output[`${reportOutputDir}/drama_${isRetryPhase ? 'retry' : 'initial'}_language_review.json`] = JSON.stringify({
        validation: config.descriptionLanguageValidation,
        checkedIds: isRetryPhase ? DRAMA_LIST : [],
        rows: collectDescriptionLanguageReview(),
    }, null, 2);
    // Keep the existing renderer unchanged while its large-report redesign is paused.
    const unescapedHtml = totalIds === 0 ? '<html><body><h2>本轮无白名单范围内的在线显示剧集</h2><p>范围外剧集未执行任何检查，不计为通过。</p></body></html>' : formatDramaReportHtml(htmlReport(data, { title: k6Config.reportTitle || 'Drama Detection Report' }), totalEpisodes, totalIds);
    if (isRetryPhase) {
        output[`${reportOutputDir}/drama_retry_report.html`] = unescapedHtml;
        output[`${reportOutputDir}/drama_retry_result.json`] = JSON.stringify({
            generatedAt: new Date().toISOString(),
            phase: 'retry',
            ruleScope: currentRuleScope(),
            retryIds,
            persistentFailures: failedChecks,
            structuredFailures,
        }, null, 2);
        return output;
    }

    output[`${reportOutputDir}/drama_check_report.html`] = unescapedHtml;
    output[`${reportOutputDir}/drama_failure_summary.json`] = JSON.stringify({
        generatedAt: new Date().toISOString(),
        phase: 'initial',
        ruleScope: currentRuleScope(),
        failures: structuredFailures,
    }, null, 2);
    output[`${reportOutputDir}/drama_retry_candidates.json`] = JSON.stringify({
        generatedAt: new Date().toISOString(),
        phase: 'initial',
        candidates: retryCandidates,
    }, null, 2);
    return output;
}

function currentRuleScope() {
    return {version:BUSINESS_RULE_VERSION,retryPolicy,metadataRuleExecution:'collection',languageDetectionExecution:'go_local',k6Execution:['description_language','playback'],groupChecks:!skipGroupChecks,ignoreNonWhitelistGroups:true,unlockTypeChecks:!skipUnlockTypeChecks,dubbingClassificationChecks:!skipDubbingClassificationChecks,dubbingCreatedAtStart:'2026-06-01',dubbingInvalidCreatedAt:'skip',businessTimezone:'+08:00',languageChecks:true,playbackChecks:true,freeChapterChecks:!skipFreeChapterChecks,copyrightChecks:!skipCopyrightChecks,filterVersion:DRAMA_SCOPE_VERSION,sourceCount:config.inspectionScope.sourceCount,includedCount:config.dramaList.length,excludedCount:config.inspectionScope.excludedIds.length};
}

function collectDescriptionLanguageReview() {
    const rows = [];
    for (const dramaId of DRAMA_LIST) {
        const meta = getRawDramaMeta(dramaId), result = meta.description_language_check || {};
        if (!['uncertain', 'mixed', 'unsupported'].includes(result.status)) continue;
        rows.push({ dramaId, intId: meta.int_id || '', title: meta.title || '', cnTitle: meta.cn_name || meta.cn_title || '', lang: meta.lang || '', detected: result.detected || '', status: result.status, reason: result.reason || '', description: meta.description || '' });
    }
    return rows;
}

function descriptionLanguageReviewPanel() {
    const escape = value => String(value || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    const labels = { uncertain: '待复核', mixed: '疑似混合', unsupported: '疑似范围外' };
    const rows = DRAMA_LIST.filter(id => labels[(getRawDramaMeta(id).description_language_check || {}).status]).map(id => {
        const meta = getRawDramaMeta(id);
        const result = meta.description_language_check;
        return `<tr><td>${escape(meta.cn_name || id)}<br>${escape(id)}</td><td>${escape(labels[result.status])}</td><td>${escape(meta.lang)} / ${escape(result.detected || '未知')}</td><td>${escape(result.reason)}</td><td><details><summary>查看简介</summary><div style="white-space:pre-wrap;overflow-wrap:anywhere;max-width:640px">${escape(meta.description)}</div></details></td></tr>`;
    });
    if (!rows.length) return '';
    return `<details><summary>待人工复核明细（${rows.length} 部，不计为通过，不自动发送飞书语种异常）</summary><div style="overflow-x:auto"><table><thead><tr><th>剧集</th><th>状态</th><th>lang / 识别</th><th>依据</th><th>简介</th></tr></thead><tbody>${rows.join('')}</tbody></table></div></details>`;
}

function formatDramaReportHtml(reportHtml, totalEpisodes, dramaCount) {
    let html = unescapeDramaReportDetails(reportHtml);
    html = moveOtherChecksBetweenRatesAndCounters(html);
    const languageSummary = config.descriptionLanguageValidation;
    const counts = languageSummary.counts || {};
    const languagePanel = `<section style="margin:16px 0;padding:16px;border:1px solid #dbeafe;border-radius:10px;background:#eff6ff"><h3>简介语种检查 · Lingua-Go（本地筛查）</h3><p>检查 ${Number(languageSummary.total || 0)} 部（含临时直接 PASS 剧集） · 通过 ${Number(counts.passed || 0)} · 空简介跳过 ${Number(counts.skipped || 0)} · 无法确定/待复核 ${Number(counts.uncertain || 0)} · 疑似混合 ${Number(counts.mixed || 0)} · 疑似范围外 ${Number(counts.unsupported || 0)} · 疑似不一致 ${Number(counts.mismatch || 0)} · lang异常 ${Number(counts.invalid_lang || 0)}</p><p>本轮元数据识别耗时 ${Number(languageSummary.duration_ms || 0)}ms · 缓存命中 ${Number(languageSummary.cache_hits || 0)}；刷新元数据不等于独立语言复核，分数不代表真实正确概率。相近语种冲突、混合、范围外与转写候选保留待复核，不自动报错；其他持续疑似异常见 Other Checks。中文不区分简繁体。</p>${descriptionLanguageReviewPanel()}</section>`;
    const scope=config.inspectionScope;
    const activeScope = `<section style="margin:16px 0;padding:14px 16px;border:1px solid #c7d2fe;border-radius:10px;background:#eef2ff"><strong>规则版本 ${BUSINESS_RULE_VERSION}</strong><p>仅15个白名单组参与；原始${scope.sourceCount}部，纳入${config.dramaList.length}部，过滤${scope.excludedIds.length}部。分组：${skipGroupChecks?'暂停':'启用'}；解锁类型：${skipUnlockTypeChecks?'暂停':'启用'}；免费集数：${skipFreeChapterChecks?'暂停':'启用'}；版权：${skipCopyrightChecks?'暂停':'启用'}；配音：${skipDubbingClassificationChecks?'暂停':'启用'}；语种与播放启用。内容五部及旧命名豁免已取消。范围外不执行任何检查，过滤不代表通过。</p></section>`;
    const activeLanguagePanel = languagePanel.replace('（含临时直接 PASS 剧集）', '（仅白名单范围内剧集）');
    const classificationDateNote = '<p style="margin:16px;color:#475569">分组校验只考虑白名单组；外部联运等非白名单组忽略，原始分组保留在筛选归档。白名单组之间仍执行互斥及独占规则。配音分类仅检查created_at为2026-06-01（含）及之后的剧集；更早、日期缺失或无法解析时仅跳过分类，其他检查正常执行。日期按业务时区+08:00判断，跳过不计为通过。</p>';
    const confirmationPolicyNote='<p style="margin:16px;color:#475569">分组、解锁类型、免费集数、版权和配音分类在收集阶段完成判断并独立保存，k6只读取这些结果并执行简介语种结果判定和播放检查。分组不符、unlock_type空/未知、免费集数及版权异常直接进入最终报告，不因此复验，也不重复验证。配音分类、简介语种、播放接口抓取失败及无法确认的分组元数据仍进行二次确认；刷新收集时仅复核待确认的字段规则，元数据恢复后补做此前无法判断的分组校验。</p>';
    html = html.replace('<section class="drama-other-checks">', `${activeScope}${classificationDateNote}${confirmationPolicyNote}${activeLanguagePanel}<section class="drama-other-checks">`);
    html = removeChecksAndGroupsTab(html);
    html = updateTotalRequestsCard(html, totalEpisodes, dramaCount);
    html = decorateCounterCountBadges(html);
    html = highlightDramaErrorKeywords(html);
    return html;
}

function getMetricCount(data, metricName) {
    const value = data.metrics && data.metrics[metricName] && data.metrics[metricName].values && data.metrics[metricName].values.count;
    return typeof value === 'number' ? value : 0;
}

function unescapeDramaReportDetails(reportHtml) {
    return reportHtml
        .replace(/&lt;details/g, '<details')
        .replace(/&lt;\/details&gt;/g, '</details>')
        .replace(/&lt;summary/g, '<summary')
        .replace(/&lt;\/summary&gt;/g, '</summary>')
        .replace(/&lt;br\s*\/?&gt;/g, '<br>')
        .replace(/&lt;b&gt;/g, '<b>')
        .replace(/&lt;\/b&gt;/g, '</b>')
        .replace(/&lt;div/g, '<div')
        .replace(/&lt;\/div&gt;/g, '</div>')
        .replace(/&#34;/g, '"')
        .replace(/&gt;/g, '>');
}

function moveOtherChecksBetweenRatesAndCounters(reportHtml) {
    const otherChecksMatch = reportHtml.match(/<h2>Other Checks<\/h2>\s*<table>[\s\S]*?<\/table>/);
    if (!otherChecksMatch) return reportHtml;

    const otherChecksBlock = `<section class="drama-other-checks">${otherChecksMatch[0]}</section>`;
    let html = reportHtml.replace(otherChecksMatch[0], '');
    const ratesBlockPattern = /(<h4><i class="fas fa-percent"><\/i> Rates<\/h4>\s*<table class="pure-table pure-table-striped">[\s\S]*?<\/table>)/;
    if (ratesBlockPattern.test(html)) {
        return html.replace(ratesBlockPattern, `$1\n${otherChecksBlock}`);
    }

    const countersHeading = '<h4><i class="fas fa-calculator"></i> Counters</h4>';
    return html.replace(countersHeading, `${otherChecksBlock}\n${countersHeading}`);
}

function removeChecksAndGroupsTab(reportHtml) {
    return reportHtml.replace(
        /\s*<input type="radio" name="tabs" id="tabthree">\s*<label for="tabthree">[\s\S]*?Checks &amp; Groups[\s\S]*?<\/label>\s*<div class="tab">[\s\S]*?<\/div>\s*<!-- ---- end tab ---- -->/m,
        ''
    ).replace(
        /\s*<input type="radio" name="tabs" id="tabthree">\s*<label for="tabthree">[\s\S]*?Checks & Groups[\s\S]*?<\/label>\s*<div class="tab">[\s\S]*?<\/div>\s*<!-- ---- end tab ---- -->/m,
        ''
    );
}

function updateTotalRequestsCard(reportHtml, totalEpisodes, dramaCount) {
    return reportHtml.replace(
        /(<div class="metric-card primary)(\">\s*<i class="fas fa-globe icon"><\/i>\s*<h4>)Total Requests(<\/h4>\s*<div class="metric-value">)\s*[\s\S]*?(\s*<\/div>\s*<\/div>)/,
        `$1 drama-total-dramas-card$2Total Dramas$3\n              <span class="drama-total-episodes">${totalEpisodes}</span><span class="drama-total-unit">（${dramaCount}部剧）</span>\n              $4`
    );
}

function decorateCounterCountBadges(reportHtml) {
    const counterRules = [
        { label: 'Converting Chapters (转换中章节总数)', type: 'errorWhenPositive' },
        { label: 'Direct Skips (直接跳集剧集数)', type: 'errorWhenPositive' },
        { label: 'Errored Chapters (出错章节总数)', type: 'errorWhenPositive' },
        { label: 'Fetch Failures (接口抓取失败数)', type: 'errorWhenPositive' },
        { label: 'Unlock Type Rule Failures (解锁类型规则异常剧集数)', type: 'errorWhenPositive' },
        { label: 'App Group Metadata Failures (App Group元数据异常剧集数)', type: 'errorWhenPositive' },
        { label: 'Naming Rule Failures (命名规范异常数)', type: 'errorWhenPositive' },
        { label: 'Classification Rule Failures (分类规则异常剧集数)', type: 'errorWhenPositive' },
        { label: 'Description Language Failures (简介语种异常剧集数)', type: 'errorWhenPositive' },
        { label: 'Free Chapter Failures (免费集数异常剧集数)', type: 'errorWhenPositive' },
        { label: 'Total Drama Episodes (总剧集数)', type: 'alwaysSuccess' },
        { label: 'Transport Retries (网络重试次数)', type: 'warningWhenPositive' },
        { label: 'Retry Candidates (二次确认候选异常数)', type: 'warningWhenPositive' },
    ];

    return reportHtml.replace(/<tr>[\s\S]*?<\/tr>/g, rowHtml => {
        const rule = counterRules.find(item => rowHtml.includes(item.label));
        if (!rule) return rowHtml;

        const cells = [...rowHtml.matchAll(/<td([^>]*)>([\s\S]*?)<\/td>/g)];
        if (cells.length === 0) return rowHtml;

        const countCell = cells[cells.length - 1];
        const countValue = parseCounterDisplayValue(countCell[2]);
        const tone = getCounterBadgeTone(rule.type, countValue);
        const className = `drama-counter-count drama-counter-count--${tone}`;
        const decoratedOpenTag = addClassToHtmlTag(`<td${countCell[1]}>`, className);
        const decoratedCell = `${decoratedOpenTag}${countCell[2]}</td>`;

        return `${rowHtml.slice(0, countCell.index)}${decoratedCell}${rowHtml.slice(countCell.index + countCell[0].length)}`;
    });
}

function parseCounterDisplayValue(rawValue) {
    const textValue = String(rawValue || '').replace(/<[^>]*>/g, '').replace(/,/g, '').trim();
    const numberValue = Number.parseFloat(textValue);
    return Number.isFinite(numberValue) ? numberValue : 0;
}

function getCounterBadgeTone(ruleType, countValue) {
    if (ruleType === 'alwaysSuccess') return 'success';
    if (ruleType === 'warningWhenPositive') return countValue >= 1 ? 'warning' : 'success';
    return countValue >= 1 ? 'danger' : 'success';
}

function addClassToHtmlTag(openTag, className) {
    if (/class="/.test(openTag)) {
        return openTag.replace(/class="([^"]*)"/, `class="$1 ${className}"`);
    }
    return openTag.replace(/>$/, ` class="${className}">`);
}

function highlightDramaErrorKeywords(reportHtml) {
    return reportHtml
        .replace(/\[出错\]/g, '<span class="drama-error-keyword">[出错]</span>')
        .replace(/\[转换中\]/g, '<span class="drama-error-keyword">[转换中]</span>')
        .replace(/\[直接跳集\]/g, '<span class="drama-error-keyword">[直接跳集]</span>')
        .replace(/\[App Group元数据\]/g, '<span class="drama-error-keyword">[App Group元数据]</span>')
        .replace(/\[解锁类型\]/g, '<span class="drama-error-keyword">[解锁类型]</span>')
        .replace(/\[分组规则\]/g, '<span class="drama-error-keyword">[分组规则]</span>')
        .replace(/\[命名规范\]/g, '<span class="drama-error-keyword">[命名规范]</span>')
        .replace(/\[分类规则\]/g, '<span class="drama-error-keyword">[分类规则]</span>')
        .replace('</head>', `<style>
  .drama-error-keyword {
    color: #dc2626;
    font-weight: 800;
  }
  .drama-total-dramas-card h4 {
    white-space: nowrap;
    font-size: clamp(0.78rem, 1.4vw, 1rem);
  }
  .drama-total-dramas-card .metric-value {
    display: flex;
    align-items: baseline;
    justify-content: center;
    gap: 0.25rem;
    flex-wrap: wrap;
    line-height: 1.08;
    word-break: keep-all;
  }
  .drama-total-dramas-card .drama-total-episodes {
    font-size: clamp(1.55rem, 4vw, 2.35rem);
    font-weight: 800;
  }
  .drama-total-dramas-card .drama-total-unit {
    font-size: clamp(0.76rem, 1.6vw, 1rem);
    font-weight: 700;
    opacity: 0.9;
    white-space: nowrap;
  }
  .drama-other-checks {
    display: block;
    margin: 1.5rem 0;
  }
  .drama-other-checks h2 {
    color: #2d3748;
    font-size: 1.25rem;
    margin: 0 0 1rem;
  }
  td.drama-counter-count {
    text-align: right;
    font-weight: 800;
    border-radius: 10px;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.55);
  }
  td.drama-counter-count--success {
    color: #166534;
    background: linear-gradient(135deg, #dcfce7 0%, #bbf7d0 100%) !important;
  }
  td.drama-counter-count--warning {
    color: #9a3412;
    background: linear-gradient(135deg, #ffedd5 0%, #fed7aa 100%) !important;
  }
  td.drama-counter-count--danger {
    color: #991b1b;
    background: linear-gradient(135deg, #fee2e2 0%, #fecaca 100%) !important;
  }
</style>
</head>`);
}

function collectRetryCandidates(data) {
    const byDrama = {};
    walkChecks(data.root_group, checkItem => {
        const name = checkItem.name || '';
        if (!checkItem.fails) return;
        const marker = parseRetryMarkerName(name);
        if (!marker || !marker.dramaId) return;
        if (!byDrama[marker.dramaId]) {
            byDrama[marker.dramaId] = { dramaId: marker.dramaId, kinds: [], reasons: [] };
        }
        if (marker.kind && !byDrama[marker.dramaId].kinds.includes(marker.kind)) {
            byDrama[marker.dramaId].kinds.push(marker.kind);
        }
        if (marker.reason && !byDrama[marker.dramaId].reasons.includes(marker.reason)) {
            byDrama[marker.dramaId].reasons.push(marker.reason);
        }
    });
    return Object.values(byDrama).map(item => ({
        dramaId: item.dramaId,
        kind: item.kinds.join(','),
        reason: item.reasons.join('；'),
    }));
}

function parseRetryMarkerName(name) {
    const value = String(name || '');
    let prefix = '';
    let legacy = false;
    if (value.startsWith(RETRY_MARKER_PREFIX)) {
        prefix = RETRY_MARKER_PREFIX;
    } else if (value.startsWith(LEGACY_RETRY_MARKER_PREFIX)) {
        prefix = LEGACY_RETRY_MARKER_PREFIX;
        legacy = true;
    } else {
        return null;
    }

    const parts = value.substring(prefix.length).split('|');
    const dramaId = parts[0] || '';
    const kind = legacy ? '720_fetch' : (parts[1] || 'unknown');
    const encodedReason = legacy ? parts.slice(1).join('|') : parts.slice(2).join('|');
    let reason = encodedReason;
    try {
        reason = decodeURIComponent(encodedReason || '');
    } catch (e) {
        // Keep malformed legacy text readable instead of dropping the retry candidate.
    }
    return { dramaId, kind, reason };
}

function isRetryMarkerName(name) {
    const value = String(name || '');
    return value.startsWith(RETRY_MARKER_PREFIX) || value.startsWith(LEGACY_RETRY_MARKER_PREFIX);
}

function countRetryCandidateChecks(data) {
    let count = 0;
    walkChecks(data.root_group, checkItem => {
        if (checkItem.fails && isRetryMarkerName(checkItem.name)) {
            count += Number(checkItem.fails) || 0;
        }
    });
    return count;
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

function collectDramaFailureSummaries(data) {
    const byDrama = {};
    walkChecks(data.root_group, checkItem => {
        const name = checkItem.name || '';
        if (!checkItem.fails || isRetryMarkerName(name)) return;

        const dramaId = extractDramaIdFromCheckName(name);
        if (!dramaId) return;

        if (!byDrama[dramaId]) {
            const meta = getDramaMeta(dramaId);
            byDrama[dramaId] = {
                dramaId,
                intId: meta.intId,
                title: meta.title,
                cnTitle: meta.cnTitle,
                errors: [],
            };
        }

        extractDramaErrorLines(name).forEach(line => {
            if (!byDrama[dramaId].errors.includes(line)) {
                byDrama[dramaId].errors.push(line);
            }
        });
    });

    return Object.values(byDrama).filter(item => item.errors.length > 0);
}

function extractDramaIdFromCheckName(name) {
    const match = String(name || '').match(/(?:剧集\s*)?ID:\s*([0-9a-fA-F]{24})/);
    return match ? match[1] : '';
}

function extractDramaErrorLines(name) {
    const plainText = decodeHTMLText(
        String(name || '')
            .replace(/<br\s*\/?>/gi, '\n')
            .replace(/<\/(summary|div|details|b)>/gi, '\n')
            .replace(/<[^>]+>/g, ' ')
    );
    const lines = [];
    const bulletPattern = /•\s*(\[[^\]]+\][^\n]+)/g;
    let match;
    while ((match = bulletPattern.exec(plainText)) !== null) {
        lines.push(`• ${normalizeWhitespace(match[1])}`);
    }
    if (lines.length > 0) return lines;

    const summaryMatch = plainText.match(/(?:剧集\s*)?ID:\s*[0-9a-fA-F]{24}[^\n]*?\s-\s*(\[[^\]]+\][\s\S]*?)(?:\n|$)/);
    if (summaryMatch) {
        return [`• ${normalizeWhitespace(summaryMatch[1])}`];
    }
    if (/720p\s*网络请求失败/.test(plainText)) {
        return ['• [接口抓取失败] 720p 网络请求失败'];
    }
    return [];
}

function decodeHTMLText(text) {
    return String(text || '')
        .replace(/&nbsp;/g, ' ')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>')
        .replace(/&amp;/g, '&')
        .replace(/&#34;/g, '"')
        .replace(/&quot;/g, '"')
        .replace(/&#39;/g, "'");
}

function normalizeWhitespace(text) {
    return String(text || '').replace(/[ \t\r\f\v]+/g, ' ').trim();
}

function stripCompletionChecks(data, passes) {
    const strip = group => {
        if (!group) return;
        if (Array.isArray(group.checks)) group.checks = group.checks.filter(item => !item.name.startsWith('__DRAMA_COMPLETED__|'));
        (group.groups || []).forEach(strip);
    };
    strip(data.root_group);
    const values = data.metrics?.checks?.values;
    if (!values) return;
    if (typeof values.passes === 'number') values.passes = Math.max(0, values.passes - passes);
    if (typeof values.count === 'number') values.count = Math.max(0, values.count - passes);
    const total = (values.passes || 0) + (values.fails || 0);
    values.rate = total ? values.passes / total : 1;
    values.value = values.rate;
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
        group.checks = group.checks.filter(checkItem => !isRetryMarkerName(checkItem.name));
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
