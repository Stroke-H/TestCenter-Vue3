import http from 'k6/http';
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

// ---------- 2. 加载配置与数据 (Config & Drama Info) ----------
const k6Config = JSON.parse(open('./k6_config.json'));
const RETRY_MARKER_PREFIX = '__DRAMA_RETRY_CANDIDATE__|';
const LEGACY_RETRY_MARKER_PREFIX = '__DRAMA_RETRY_720_NETWORK__|';
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
const DRAMA_META = config.dramaMeta || {};
const APP_GROUPS = config.appGroups || [];
const VALIDATION_META = config.validationMeta || {};
const APP_GROUP_VALIDATION_META = VALIDATION_META.appGroups || {};
const APP_GROUP_METADATA_AVAILABLE = typeof APP_GROUP_VALIDATION_META.available === 'boolean'
    ? APP_GROUP_VALIDATION_META.available
    : APP_GROUPS.length > 0;
const APP_GROUP_METADATA_ERROR = String(APP_GROUP_VALIDATION_META.error || '').trim();
const TOKEN = config.auth.x_token;
const API_BASE = config.apiBase || "http://35.225.224.94:8080";
const DS_EXCLUSIVE_GROUP_NAME = 'TT-Minis 分销组';
const MD_PRODUCT_GROUP_NAME = 'MD-产品组';
const NEW_AD_UNLOCK_GROUP_NAMES = ['网赚短剧', 'TT_IAA_媒资库组'];
const COIN_UNLOCK_GROUP_NAME = 'TT_IAP_媒资库组';
const AD_UNLOCK_ALLOWED_GROUP_NAMES = ['IAA组', '漫剧产品组-IAA', 'AIGC组', '内容二组-觉醒纪元', DS_EXCLUSIVE_GROUP_NAME, ...NEW_AD_UNLOCK_GROUP_NAMES];
const SUPPORTED_UNLOCK_TYPES = ['coin', 'coins', 'ad'];
const DUBBING_MARKETING_POSITION = '配音剧';
const CLASSIFICATION_CHECK_START_DATE = '2026-06-01';
const DUBBING_LANGUAGE_PREFIXES = [
    '印度尼西亚', '马来西亚', '罗马尼亚', '澳大利亚', '意大利', '西班牙',
    '葡萄牙', '菲律宾', '土耳其', '阿拉伯', '墨西哥', '加拿大', '新加坡',
    '匈牙利', '乌克兰', '以色列', '希伯来', '柬埔寨', '俄罗斯', '美国',
    '英国', '法国', '德国', '印度', '日本', '韩国', '泰国', '越南', '印尼',
    '巴西', '荷兰', '波兰', '希腊', '捷克', '瑞典', '挪威', '丹麦', '芬兰',
    '沙特', '伊朗', '南非', '埃及', '马来', '缅甸', '老挝', '中文', '国语',
    '西', '葡', '英', '法', '德', '意', '日', '韩', '泰', '越', '印', '菲',
    '俄', '阿', '土', '巴', '墨', '加', '澳', '新', '马', '缅', '柬', '老',
    '荷', '波', '希', '罗', '匈', '捷', '瑞', '挪', '丹', '芬', '乌', '以',
    '沙', '伊', '南', '埃', '粤', '国', '中'
];
const CANONICAL_RULE_GROUP_NAMES = Array.from(new Set([
    ...AD_UNLOCK_ALLOWED_GROUP_NAMES,
    MD_PRODUCT_GROUP_NAME,
    COIN_UNLOCK_GROUP_NAME,
]));

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

function buildUnlockTypeConfirmationIssue(dramaId) {
    const meta = getDramaMeta(dramaId);
    const unlockType = meta.unlockType.toLowerCase();
    if (!SUPPORTED_UNLOCK_TYPES.includes(unlockType)) {
        return unlockType ? `unlock_type 未知值: ${meta.unlockType}` : 'unlock_type 为空';
    }
    return '';
}

function buildUnlockTypeRuleErrors(dramaId, includeAppGroupMetadataError) {
    const meta = getDramaMeta(dramaId);
    const cnNameHasIAA = hasLooseIAA(meta.cnName);
    const cnNameHasDS = hasLooseDSMarker(meta.cnName);
    const appGroupIds = normalizeAppGroupIds(meta.appGroups);
    const groupNames = getMatchedAppGroupNames(meta.appGroups);
    const groupMetadataAvailable = APP_GROUP_METADATA_AVAILABLE;
    const groupHasIAA = groupNames.some(hasLooseIAA);
    const groupHasShortsWave = groupNames.some(hasLooseShortsWave);
    const groupHasAllowedAdUnlockName = groupNames.some(name => AD_UNLOCK_ALLOWED_GROUP_NAMES.some(canonical => matchesCanonicalGroupName(name, canonical)));
    const hasOnlyDSExclusiveGroup = appGroupIds.length === 1
        && groupNames.length === 1
        && matchesCanonicalGroupName(groupNames[0], DS_EXCLUSIVE_GROUP_NAME);
    const hasOnlyMDProductGroup = appGroupIds.length === 1
        && groupNames.length === 1
        && matchesCanonicalGroupName(groupNames[0], MD_PRODUCT_GROUP_NAME);
    const unlockType = meta.unlockType.toLowerCase();
    const errors = buildNamingRuleErrors(meta, groupNames);

    if (isRetryPhase && includeAppGroupMetadataError && !groupMetadataAvailable) {
        errors.push({
            type: 'metadata',
            msg: `[App Group元数据] 二次确认后仍不可用，无法完成分组与解锁类型校验${APP_GROUP_METADATA_ERROR ? `。原因=${APP_GROUP_METADATA_ERROR}` : ''}`,
            count: 1
        });
    }

    if (isRetryPhase && !SUPPORTED_UNLOCK_TYPES.includes(unlockType)) {
        errors.push({
            type: 'unlock',
            msg: `[解锁类型] 二次确认后 unlock_type ${unlockType ? `仍为未知值 ${formatEmpty(meta.unlockType)}` : '仍为空'}`,
            count: 1
        });
    }

    if (cnNameHasDS && groupMetadataAvailable && !hasOnlyDSExclusiveGroup) {
        errors.push({
            type: 'unlock',
            msg: `[分组规则] cn_name 包含 -DS 时，App Group 必须且只能为 ${DS_EXCLUSIVE_GROUP_NAME}。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`,
            count: 1
        });
    }

    if (['coin', 'coins'].includes(unlockType) && (cnNameHasIAA || groupHasIAA)) {
        errors.push({
            type: 'unlock',
            msg: `[解锁类型] unlock_type=coin，但 cn_name 或 App Group 命中 IAA。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`,
            count: 1
        });
    }

    if (!cnNameHasDS && groupMetadataAvailable && unlockType === 'ad' && groupHasShortsWave) {
        errors.push({
            type: 'unlock',
            msg: `[解锁类型] unlock_type=ad，但 App Group 命中 ShortsWave。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`,
            count: 1
        });
    } else if (!cnNameHasDS && groupMetadataAvailable && unlockType === 'ad' && !groupHasAllowedAdUnlockName && !hasOnlyMDProductGroup) {
        errors.push({
            type: 'unlock',
            msg: `[解锁类型] unlock_type=ad，但 App Group 未命中允许的广告解锁分组（${AD_UNLOCK_ALLOWED_GROUP_NAMES.join('、')}）。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`,
            count: 1
        });
    }

    // 新组独立校验，避免多分组中的广告白名单或 DS 分支绕过要求。
    // 空值和未知值仍由已有二次确认流程处理。
    if (groupMetadataAvailable && SUPPORTED_UNLOCK_TYPES.includes(unlockType)) {
        const newAdGroups = groupNames.filter(name => NEW_AD_UNLOCK_GROUP_NAMES.some(canonical => matchesCanonicalGroupName(name, canonical)));
        if (newAdGroups.length > 0 && unlockType !== 'ad' && !(cnNameHasIAA || groupHasIAA)) {
            errors.push({ type: 'unlock', msg: `[解锁类型] App Group 命中 ${newAdGroups.join('、')}，要求 unlock_type=ad，实际=${meta.unlockType}。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`, count: 1 });
        }
        if (unlockType === 'ad' && groupNames.some(name => matchesCanonicalGroupName(name, COIN_UNLOCK_GROUP_NAME))) {
            // 使用明确的付费组错误替代本次通用广告分组错误，避免重复上报。
            const genericIndex = errors.findIndex(error => error.type === 'unlock' && error.msg.includes('App Group 未命中允许的广告解锁分组'));
            if (genericIndex >= 0) errors.splice(genericIndex, 1);
            errors.push({ type: 'unlock', msg: `[解锁类型] App Group 命中 ${COIN_UNLOCK_GROUP_NAME}，要求 unlock_type=coins（兼容 coin），实际=${meta.unlockType}。cn_name=${formatEmpty(meta.cnName)}，app_groups=${formatGroupNames(groupNames)}`, count: 1 });
        }
    }
    return errors;
}

function buildMarketingPositionRuleErrors(dramaId) {
    const meta = getDramaMeta(dramaId);
    if (!isClassificationCheckDateEligible(meta.createdAt) || meta.cnName.includes('番茄')) {
        return [];
    }
    const dubbingMarker = findDubbingNameMarker(meta.cnName);
    const isDubbingCategory = meta.marketingPosition === DUBBING_MARKETING_POSITION;

    if (dubbingMarker && !isDubbingCategory) {
        return [{
            type: 'classification',
            msg: `[分类规则] cn_name 包含配音标识 ${dubbingMarker}，但短剧分类 marketing_position 未命中配音剧。cn_name=${formatEmpty(meta.cnName)}，marketing_position=${formatEmpty(meta.marketingPosition)}`,
            count: 1
        }];
    }
    if (!dubbingMarker && isDubbingCategory) {
        return [{
            type: 'classification',
            msg: `[分类规则] marketing_position=配音剧，但 cn_name 未包含 X配 标识。cn_name=${formatEmpty(meta.cnName)}`,
            count: 1
        }];
    }
    return [];
}

function isClassificationCheckDateEligible(value) {
    const dateMatch = String(value || '').trim().match(/^(\d{4}-\d{2}-\d{2})/);
    if (!dateMatch) return false;
    return dateMatch[1] >= CLASSIFICATION_CHECK_START_DATE;
}

function findDubbingNameMarker(value) {
    const text = String(value || '');
    for (const prefix of DUBBING_LANGUAGE_PREFIXES) {
        const candidates = [`${prefix}配`];
        if (!prefix.endsWith('语')) {
            candidates.push(`${prefix}语配`);
        }
        for (const candidate of candidates) {
            const index = text.indexOf(candidate);
            if (index >= 0) {
                return text[index + candidate.length] === '音' ? `${candidate}音` : candidate;
            }
        }
    }

    const latinMarker = text.match(/(?:^|[^A-Za-z])([A-Za-z]{1,4}配)(?=$|[^A-Za-z])/i);
    if (latinMarker) {
        return latinMarker[1];
    }
    return '';
}

function hasStrictIAA(value) {
    return /(^|[^A-Za-z0-9])IAA([^A-Za-z0-9]|$)/.test(String(value || ''));
}

function hasLooseIAA(value) {
    return /(^|[^A-Za-z0-9])iaa([^A-Za-z0-9]|$)/i.test(String(value || ''));
}

function hasLooseDSMarker(value) {
    return /(^|[^A-Za-z0-9])ds(?=$|[^A-Za-z0-9])/i.test(String(value || ''));
}

function hasLooseShortsWave(value) {
    return normalizeGroupNameForFuzzyRule(value).includes('shortswave');
}

function buildNamingRuleErrors(meta, groupNames) {
    const errors = [];
    const seen = {};
    const add = message => {
        if (seen[message]) return;
        seen[message] = true;
        errors.push({ type: 'naming', msg: message, count: 1 });
    };

    if (hasLooseIAA(meta.cnName) && !hasStrictIAA(meta.cnName)) {
        add(`[命名规范] cn_name 中 IAA 大小写不规范，应使用 IAA。cn_name=${formatEmpty(meta.cnName)}`);
    }
    groupNames.forEach(groupName => {
        let hasSpecificNamingIssue = false;
        if (hasLooseIAA(groupName) && !hasStrictIAA(groupName)) {
            add(`[命名规范] App Group 中 IAA 大小写不规范，应使用 IAA。app_group=${groupName}`);
            hasSpecificNamingIssue = true;
        }
        if (hasLooseShortsWave(groupName) && !String(groupName).includes('ShortsWave')) {
            add(`[命名规范] App Group 中 ShortsWave 命名大小写或分隔不规范。app_group=${groupName}`);
            hasSpecificNamingIssue = true;
        }

        const canonicalName = findClosestCanonicalGroupName(groupName, CANONICAL_RULE_GROUP_NAMES);
        if (!hasSpecificNamingIssue && canonicalName && normalizeGroupNameForRule(groupName) !== normalizeGroupNameForRule(canonicalName)) {
            add(`[命名规范] App Group 名称与标准名称相近，建议修正为 ${canonicalName}。当前=${groupName}`);
        }
    });
    return errors;
}

function matchesCanonicalGroupName(value, canonicalName) {
    if (NEW_AD_UNLOCK_GROUP_NAMES.includes(canonicalName) || canonicalName === COIN_UNLOCK_GROUP_NAME) {
        // IAA 与 IAP 只差一个字母，必须在完整名单中优先选最接近的标准组。
        return findClosestCanonicalGroupName(value, CANONICAL_RULE_GROUP_NAMES) === canonicalName;
    }
    return findClosestCanonicalGroupName(value, [canonicalName]) === canonicalName;
}

function findClosestCanonicalGroupName(value, canonicalNames) {
    const source = normalizeGroupNameForFuzzyRule(value);
    if (!source) return '';

    let bestName = '';
    let bestDistance = Number.POSITIVE_INFINITY;
    (canonicalNames || []).forEach(canonicalName => {
        const target = normalizeGroupNameForFuzzyRule(canonicalName);
        if (!target) return;
        const distance = source === target ? 0 : levenshteinDistance(source, target);
        const threshold = target.length >= 10 ? 2 : 1;
        if (distance <= threshold && distance < bestDistance) {
            bestDistance = distance;
            bestName = canonicalName;
        }
    });
    return bestName;
}

function normalizeGroupNameForFuzzyRule(value) {
    return String(value || '')
        .toLowerCase()
        .replace(/[\s\u00a0\-_—–·.，,()（）\[\]【】]+/g, '');
}

function levenshteinDistance(left, right) {
    const rows = Array.from({ length: left.length + 1 }, (_, index) => index);
    for (let column = 1; column <= right.length; column++) {
        let previousDiagonal = rows[0];
        rows[0] = column;
        for (let row = 1; row <= left.length; row++) {
            const previousRowValue = rows[row];
            const substitutionCost = left[row - 1] === right[column - 1] ? 0 : 1;
            rows[row] = Math.min(
                rows[row] + 1,
                rows[row - 1] + 1,
                previousDiagonal + substitutionCost
            );
            previousDiagonal = previousRowValue;
        }
    }
    return rows[left.length];
}

function getMatchedAppGroupNames(appGroups) {
    const ids = normalizeAppGroupIds(appGroups);
    if (ids.length === 0 || !Array.isArray(APP_GROUPS)) return [];
    return APP_GROUPS
        .filter(group => ids.includes(getAppGroupId(group)))
        .map(group => String(group && (group.name || group.group_name || group.groupName || group.title) || '').trim())
        .filter(Boolean);
}

function normalizeAppGroupIds(value) {
    if (Array.isArray(value)) {
        return value.map(item => String(item && (item._id || item.id || item.group_id || item.groupId || item.value || item.key) || item).trim()).filter(Boolean);
    }
    if (value && typeof value === 'object') {
        return [String(value._id || value.id || value.group_id || value.groupId || value.value || value.key || '').trim()].filter(Boolean);
    }
    const text = String(value || '').trim();
    if (!text) return [];
    try {
        return normalizeAppGroupIds(JSON.parse(text));
    } catch (e) {
        return text.split(/[,\s|;]+/).map(item => item.trim()).filter(Boolean);
    }
}

function getAppGroupId(group) {
    return String(group && (group.id || group._id || group.group_id || group.groupId || group.value || group.key) || '').trim();
}

function formatGroupNames(groupNames) {
    return groupNames.length > 0 ? groupNames.join(', ') : '未匹配';
}

function normalizeGroupNameForRule(value) {
    return String(value || '').replace(/[\s\u00a0]+/g, '');
}

function formatEmpty(value) {
    const text = String(value || '').trim();
    return text || '空';
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
    const meta = DRAMA_META[dramaId] || {};
    return {
        intId: String(meta.int_id || meta.intId || '').trim(),
        title: String(meta.title || meta.name || '').trim(),
        cnTitle: String(meta.cn_title || meta.cnTitle || meta.cn_name || meta.cnName || '').trim(),
        cnName: String(meta.cn_name || meta.cnName || meta.cn_title || meta.cnTitle || '').trim(),
        createdAt: String(meta.created_at || meta.createdAt || '').trim(),
        unlockType: String(meta.unlock_type || meta.unlockType || '').trim(),
        marketingPosition: String(meta.marketing_position || meta.marketingPosition || '').trim(),
        appGroups: meta.app_groups || meta.appGroups || meta.app_group_ids || meta.appGroupIds || [],
    };
}

// ---------- 6. 主执行逻辑 ----------
export default function () {
    const index = (__VU - 1) * itersPerVu + __ITER;
    if (index >= totalIds) return;
    const dramaId = DRAMA_LIST[index];
    if (!dramaId) return;

    const ownsGlobalMetadataConfirmation = index === 0;
    if (!isRetryPhase && ownsGlobalMetadataConfirmation && !APP_GROUP_METADATA_AVAILABLE) {
        const reason = `App Group 元数据不可用${APP_GROUP_METADATA_ERROR ? `: ${APP_GROUP_METADATA_ERROR}` : ''}`;
        queueSecondConfirmation(dramaId, 'app_group_metadata', [reason]);
        console.warn(`[↻] ${reason}，加入全局二次确认队列。`);
    }
    const unlockTypeConfirmationIssue = buildUnlockTypeConfirmationIssue(dramaId);
    if (!isRetryPhase && unlockTypeConfirmationIssue) {
        queueSecondConfirmation(dramaId, 'unlock_type', [unlockTypeConfirmationIssue]);
        console.warn(`[↻] 剧集 ${dramaId} ${unlockTypeConfirmationIssue}，加入二次确认队列。`);
    }
    const validationRuleErrors = [
        ...buildUnlockTypeRuleErrors(dramaId, ownsGlobalMetadataConfirmation),
        ...buildMarketingPositionRuleErrors(dramaId),
    ];

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
        if (err.type === 'fetch') fetchErrors.add(err.count);
    });
    const summaryLabel = buildDramaSummaryLabel(dramaId, errors, true);
    const detailLines = errorDetails.join('<br>');
    const expandableMsg = `<details style="cursor: pointer;"><summary><b>${summaryLabel}</b></summary><div style="margin-left: 20px; padding: 5px; border-left: 2px solid #eee; font-size: 0.9em;">${detailLines}</div></details>`;
    console.warn(`[❌ 实锤故障] ${formatDramaIdentityPlain(dramaId)}\n  ${errorDetails.join('\n  ')}`);
    check(null, { [expandableMsg]: false });
}

export function handleSummary(data) {
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
    };

    const finalMetrics = {};
    Object.keys(data.metrics).forEach(key => {
        const translatedName = nameMap[key] || key;
        finalMetrics[translatedName] = data.metrics[key];
    });
    data.metrics = finalMetrics;

    const reportHtml = htmlReport(data, { title: k6Config.reportTitle || "Drama Detection Report (核心剧集章节检测报表)" });
    const unescapedHtml = formatDramaReportHtml(reportHtml, totalEpisodes, totalIds);

    const output = {};
    if (isRetryPhase) {
        output[`${reportOutputDir}/drama_retry_report.html`] = unescapedHtml;
        output[`${reportOutputDir}/drama_retry_result.json`] = JSON.stringify({
            generatedAt: new Date().toISOString(),
            phase: 'retry',
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
        failures: structuredFailures,
    }, null, 2);
    output[`${reportOutputDir}/drama_retry_candidates.json`] = JSON.stringify({
        generatedAt: new Date().toISOString(),
        phase: 'initial',
        candidates: retryCandidates,
    }, null, 2);
    return output;
}

function formatDramaReportHtml(reportHtml, totalEpisodes, dramaCount) {
    let html = unescapeDramaReportDetails(reportHtml);
    html = moveOtherChecksBetweenRatesAndCounters(html);
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
