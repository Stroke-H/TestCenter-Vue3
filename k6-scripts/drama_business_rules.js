// Pure business rules shared by k6 and local regression tests. No HTTP calls.
import { inspectionGroupNames } from './drama_scope.js';
export const BUSINESS_RULE_VERSION = '2026-11-06-v4.7';
export const DUBBING_CHECK_START = '2026-06-01';
export const ORDINARY_AD_GROUP_NAMES = ['IAA组', '漫剧产品组-IAA', '网赚短剧'];
export const normalizeRuleName = value => String(value ?? '').replace(/（/g, '(').replace(/）/g, ')');
const groupKey = value => normalizeRuleName(value).replace(/\s+/g, '').toLowerCase();
const error = (type, msg) => ({ type, msg, count: 1 });
const languageKey = value => {
  const base = String(value ?? '').trim().toLowerCase().replace(/_/g, '-').split('-')[0];
  return ({ in: 'id', tw: 'zh' })[base] || base;
};

export function checkDramaGroups(meta, names, available) {
  // The allowlist determines both participation and the groups that constrain
  // this check. Keep raw membership outside this pure validator for auditing.
  names = inspectionGroupNames(names);
  const unlock = meta.unlockType;
  const errors = [];
  if (!['coin', 'ad'].includes(unlock)) {
    errors.push(error('unlock', `[解锁类型] unlock_type 仅允许 coin/ad，实际=${unlock || '空'}。`));
  }
  if (!available) {
    errors.push(error('metadata', '[App Group元数据] 分组元数据缺失或存在无法解析的分组ID，无法安全校验。'));
    return errors;
  }
  const name = normalizeRuleName(meta.cnName);
  const targets = [];
  // Copyright's exclusive group is the highest-priority group rule.
  if (String(meta.copyright ?? '').trim() === '番茄内供') {
    if(names.length!==1 || groupKey(names[0])!==groupKey('ShortsWave 组')) errors.push(error('unlock',`[分组规则] copyright=番茄内供，要求有且只有ShortsWave 组，实际=${names.join('、')||'无分组'}。`));
    return errors;
  }
  if(!['coin','ad'].includes(unlock))return errors;
  // Special names replace ordinary membership rules. Conflicting targets are
  // reported rather than silently choosing whichever rule happens to be first.
  if (name.includes('互动')) targets.push('海外互动短剧');
  if (unlock === 'ad' && name.includes('(出海)')) targets.push('点石出海短剧');
  if (name.includes('媒资库')) {
    if (/-DS媒资库/i.test(name)) targets.push(name.includes('(东南亚)')?'TT_IAA_分销_媒资库（香港主体）':'TT_IAA_分销_媒资库');
    else if (name.includes('(东南亚)')) targets.push('TT_IAA_媒资库组（香港主体）');
    else if (unlock === 'ad' && /\(IAA\)/i.test(name) && name.includes('(媒资库)') && name.includes('(香港主体)')) targets.push('TT_IAA_媒资库组（香港主体）');
    else if (name.includes('(美欧)')) targets.push('TT_IAA_媒资库组');
    else if (unlock === 'ad' && /\(IAA\)/i.test(name)) targets.push('TT_IAA_媒资库组');
    else if (unlock === 'coin' && /\(媒资库\)\s*$/.test(name)) targets.push('TT_IAP_媒资库组');
  }
  const uniqueTargets = [...new Set(targets)];
  if (uniqueTargets.length > 1) return [error('unlock', `[分组规则] 特殊命名冲突，要求不同独占分组：${uniqueTargets.join('、')}。cn_name=${meta.cnName}`)];
  const actual = names.map(groupKey);
  if (uniqueTargets.length) {
    if (actual.length !== 1 || actual[0] !== groupKey(uniqueTargets[0])) {
      errors.push(error('unlock', `[分组规则] 特殊命名要求有且只有 ${uniqueTargets[0]}，实际=${names.join('、') || '无分组'}。cn_name=${meta.cnName}`));
    }
    return errors;
  }
  if (unlock === 'coin') {
    const allowed=['ShortsWave 组','小程序组','mangogo组','CoCoShort产品组'].map(groupKey);
    if (!actual.length || !actual.every(n=>allowed.includes(n))) errors.push(error('unlock', `[分组规则] 普通coin仅允许ShortsWave/小程序/mangogo/CoCoShort四组的任意非空组合，不得包含其他白名单组。实际参与校验=${names.join('、')||'无分组'}。`));
  } else {
    const minis=groupKey('TT-Minis 分销组');
    const exactMinis=actual.length===1&&actual[0]===minis;
    const adKeys=ORDINARY_AD_GROUP_NAMES.map(groupKey);
    const exactAds=actual.length>0&&actual.every(n=>adKeys.includes(n));
    if(!exactMinis&&!exactAds) errors.push(error('unlock',`[分组规则] 普通ad仅允许独占TT-Minis 分销组，或IAA/漫剧产品组-IAA/网赚短剧的任意非空组合；两类互斥且不得含其他白名单组。实际参与校验=${names.join('、')||'无分组'}。`));
  }
  return errors;
}

const dubbingLanguages = {
  en: ['英语', '英文', '英'], ja: ['日语', '日文', '日本', '日'], es: ['西班牙语', '西班牙', '西语', '西'],
  id: ['印度尼西亚', '印度尼西亚语', '印尼语', '印尼', '印'], fr: ['法语', '法国', '法'],
  ko: ['韩语', '韩国', '韩'], th: ['泰语', '泰国', '泰'], ru: ['俄语', '俄罗斯', '俄'],
  ms: ['马来西亚', '马来语', '马来', '马'], zh: ['中文', '国语', '中', '国'],
  ar: ['阿拉伯语', '阿拉伯', '沙特', '阿'], pt: ['葡萄牙语', '葡萄牙', '葡语', '葡', '巴西'],
  de: ['德语', '德国', '德'], tr: ['土耳其语', '土耳其', '土']
};
const prefixLanguage = Object.fromEntries(Object.entries(dubbingLanguages).flatMap(([code, prefixes]) => prefixes.map(p => [p, code])));
const dubbingPattern = new RegExp(`(${Object.keys(prefixLanguage).sort((a, b) => b.length - a.length).join('|')})(?:全剧配音|配)`, 'g');

export function checkDramaDubbing(meta) {
  const created=parseBusinessDateTime(meta.createdAt ?? meta.created_at);
  if(!Number.isFinite(created)||created<Date.parse(`${DUBBING_CHECK_START}T00:00:00+08:00`))return [];
  const markers = [...normalizeRuleName(meta.cnName).matchAll(dubbingPattern)];
  const languages = [...new Set(markers.map(m => prefixLanguage[m[1]]))];
  const category = String(meta.marketingPosition ?? '').split(/[,，、|;；\s]+/).includes('配音剧');
  const errors = [];
  const unknownTags = [...normalizeRuleName(meta.cnName).matchAll(/\((?:AI)?([一-龥]{1,8}?)(?:全剧配音|配)[^()]*\)/g)]
    .filter(m => !Object.keys(prefixLanguage).some(prefix => m[1].endsWith(prefix)));
  if (unknownTags.length) errors.push(error('classification', `[分类规则] 配音标识尚未配置语种映射：${unknownTags.map(m => m[1] + '配').join('、')}，需人工核验。`));
  if (unknownTags.length && !markers.length && !category) errors.push(error('classification', '[分类规则] 命名存在配音标识，但 marketing_position 未包含配音剧。'));
  if (markers.length && !category) errors.push(error('classification', `[分类规则] cn_name 含 ${markers.map(m => m[0]).join('、')}，marketing_position 未包含配音剧。实际=${meta.marketingPosition || '空'}。`));
  if (languages.length > 1) errors.push(error('classification', `[分类规则] 命名同时包含不同语种的配音标识：${markers.map(m => m[0]).join('、')}。`));
  else if (languages.length && languageKey(meta.lang) !== languages[0]) errors.push(error('classification', `[分类规则] ${markers[0][0]} 应对应 lang=${languages[0]}，实际 lang=${meta.lang || '空'}。`));
  return errors;
}

export function checkDramaFreeChapters(meta) {
  if (meta.unlockType !== 'ad') return [];
  const value = meta.freeChapters;
  const valid = meta.freeChaptersAvailable && ((typeof value === 'number' && Number.isInteger(value) && value >= 0) || (typeof value === 'string' && /^\d+$/.test(value.trim())));
  if (!valid) return [error('free_chapters', '[免费集数] 后台 free_chapters 缺失或格式无效，不能视为0或通过。')];
  if (Number(value) !== 2) return [error('free_chapters', `[免费集数] unlock_type=ad 要求 free_chapters=2，实际=${value}。`)];
  return [];
}

export function parseBusinessDateTime(value) {
  if(typeof value==='number'&&Number.isFinite(value)&&value>0)return value<1e12?value*1000:value;
  if(typeof value!=='string'||!value.trim())return NaN;
  const raw=value.trim();
  if(/^\d{10}(?:\d{3})?$/.test(raw))return parseBusinessDateTime(Number(raw));
  const m=raw.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2}):(\d{2})(\.\d{1,3})?)?(Z|[+-]\d{2}:\d{2})?$/);
  if(!m)return NaN;
  const [y,mo,d,h,mi,s]=[m[1],m[2],m[3],m[4]||0,m[5]||0,m[6]||0].map(Number);
  const check=new Date(Date.UTC(y,mo-1,d,h,mi,s));
  if(check.getUTCFullYear()!==y||check.getUTCMonth()!==mo-1||check.getUTCDate()!==d||h>23||mi>59||s>59)return NaN;
  // Management API returns YYYY-MM-DD HH:mm:ss without an offset: business TZ +08:00.
  return Date.parse(`${m[1]}-${m[2]}-${m[3]}T${m[4]||'00'}:${m[5]||'00'}:${m[6]||'00'}${m[7]||''}${m[8]||'+08:00'}`);
}

export const parseCopyrightExpiry = parseBusinessDateTime;

export function checkDramaCopyright(meta,now=Date.now()) {
  const raw=meta.copyrightExpireAt;
  const expiry=parseCopyrightExpiry(raw);
  if(!Number.isFinite(expiry))return [error('copyright','[版权规则] copyright_expire_at 缺失或格式无效，无法确认版权有效。')];
  if(expiry<=now)return [error('copyright',`[版权规则] 版权已过期，copyright_expire_at=${raw}，检查时间=${new Date(now).toISOString()}，截止时间必须严格晚于当前时间。`)];
  return [];
}
