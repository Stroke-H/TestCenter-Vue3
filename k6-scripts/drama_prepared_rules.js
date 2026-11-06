// Run metadata-only rules once during collection, never once per k6 VU.
import { BUSINESS_RULE_VERSION, checkDramaGroups, checkDramaDubbing, checkDramaFreeChapters, checkDramaCopyright } from './drama_business_rules.js';
import { inspectionGroupNames } from './drama_scope.js';

export const RULE_SWITCHES = ['skipGroupChecks', 'skipUnlockTypeChecks', 'skipFreeChapterChecks', 'skipCopyrightChecks', 'skipDubbingClassificationChecks'];

export function ruleSettings(config = {}) {
  return Object.fromEntries(RULE_SWITCHES.map(key => [key, config[key] === true]));
}

function toRuleMeta(meta) {
  return {
    cnName: String(meta.cn_name || meta.cnName || meta.cn_title || meta.cnTitle || '').trim(),
    createdAt: String(meta.created_at || meta.createdAt || '').trim(),
    unlockType: String(meta.unlock_type || meta.unlockType || '').trim(),
    marketingPosition: String(meta.marketing_position || meta.marketingPosition || '').trim(),
    lang: String(meta.lang ?? '').trim(),
    freeChapters: meta.free_chapters,
    freeChaptersAvailable: meta.free_chapters_available === true,
    copyright: meta.copyright,
    copyrightExpireAt: meta.copyright_expire_at,
  };
}

export function prepareDramaRuleChecks(ids, metadata, groups, available, config = {}, phase = 'initial', now = Date.now(), retryContext = {}) {
  const settings = ruleSettings(config);
  const groupNames = new Map(groups.map(group => [String(group.id), String(group.name || '').trim()]));
  const isRetry = phase === 'retry';
  const candidateIds = new Set(retryContext.candidateIds || ids);
  const metadataRetryIds = new Set(retryContext.metadataRetryIds || []);
  const failures = [], candidates = [];
  for (const id of ids) {
    const raw = metadata[id];
    if (!raw) throw new Error(`剧集 ${id} 缺少规则检查元数据`);
    const meta = toRuleMeta(raw);
    const active = !isRetry || candidateIds.has(id);
    const memberships = raw.app_groups || [];
    const names = [...groupNames].filter(([key]) => memberships.includes(key)).map(([, name]) => name).filter(Boolean);
    const complete = available && names.length === memberships.length;
    const checkedNames = inspectionGroupNames(names);
    const ignoredNames = names.filter(name => !checkedNames.includes(name));
    let groupErrors = [];
    if (!active || (isRetry && (settings.skipGroupChecks || !metadataRetryIds.has(id)))) {
      // Known group/unlock findings are not re-evaluated during confirmation.
    } else if (settings.skipGroupChecks) {
      if (!settings.skipUnlockTypeChecks && !['coin', 'ad'].includes(meta.unlockType)) {
        groupErrors = [{ type: 'unlock', count: 1, msg: `[解锁类型] unlock_type 仅允许 coin/ad，实际=${meta.unlockType || '空'}` }];
      }
    } else if (!(settings.skipUnlockTypeChecks && !['coin', 'ad'].includes(meta.unlockType))) {
      groupErrors = checkDramaGroups(meta, names, complete).map(error => ignoredNames.length
        ? { ...error, msg: `${error.msg} 已忽略非白名单组=${ignoredNames.join('、')}；完整分组=${names.join('、')}。` }
        : error);
    }
    const directErrors = isRetry ? [] : [
      ...groupErrors.filter(error => error.type !== 'metadata'),
      ...(settings.skipFreeChapterChecks ? [] : checkDramaFreeChapters(meta)),
      ...(settings.skipCopyrightChecks ? [] : checkDramaCopyright(meta, now)),
    ];
    const metadataErrors = !active || settings.skipGroupChecks || complete ? [] : !isRetry ? groupErrors.filter(error => error.type === 'metadata') : [{
      type: 'metadata', count: 1,
      msg: phase === 'retry'
        ? '[App Group元数据] 二次确认后分组元数据仍缺失或存在无法解析的分组ID，无法安全校验。'
        : '[App Group元数据] 分组元数据缺失或存在无法解析的分组ID，无法安全校验。',
    }];
    const classificationErrors = !active || settings.skipDubbingClassificationChecks ? [] : checkDramaDubbing(meta);
    raw.prepared_rules = {
      directErrors, metadataErrors, classificationErrors,
      // Only consumed for an ID whose initial group metadata was uncertain.
      resolvedGroupErrors: isRetry ? groupErrors.filter(error => error.msg.startsWith('[分组规则]')) : [],
    };
    if (directErrors.length) failures.push({ dramaId: id, intId: raw.int_id, title: raw.title, cnTitle: raw.cn_title || raw.cn_name, errors: directErrors.map(error => error.msg) });
    for (const [kind, errors] of [['app_group_metadata_case', metadataErrors], ['dubbing_classification', classificationErrors]]) {
      if (errors.length) candidates.push({ dramaId: id, kind, reason: errors.map(error => error.msg).join('；') });
    }
  }
  return {
    ready: true, version: BUSINESS_RULE_VERSION, phase, settings, checkedCount: ids.length,
    generatedAt: new Date(now).toISOString(), failures, candidates,
  };
}
