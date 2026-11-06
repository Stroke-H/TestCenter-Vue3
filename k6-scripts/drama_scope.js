export const DRAMA_SCOPE_VERSION = '2026-11-05-v4';
export const INSPECTION_GROUP_NAMES = [
  'ShortsWave 组', '小程序组', 'mangogo 组', 'CoCoShort 产品组', 'TT_IAP_媒资库组',
  'TT-Minis 分销组', '点石出海短剧', 'TT_IAA_媒资库组', 'TT_IAA_媒资库组（香港主体）',
  '海外互动短剧', 'IAA 组', '漫剧产品组 - IAA', '网赚短剧', 'TT_IAA_分销_媒资库', 'TT_IAA_分销_媒资库（香港主体）'
];
export const scopeGroupKey = value => String(value ?? '').replace(/（/g,'(').replace(/）/g,')').replace(/\s+/g,'').toLowerCase();
export function inspectionGroupNames(names) {
  return names.filter(name=>INSPECTION_GROUP_NAMES.some(allowed=>scopeGroupKey(allowed)===scopeGroupKey(name)));
}
export function scopeGroupIds(value) {
  if(Array.isArray(value)) return [...new Set(value.map(item=>String(item?.id ?? item?._id ?? item?.group_id ?? item?.value ?? item).trim()).filter(Boolean))];
  if(value && typeof value==='object') return scopeGroupIds([value]);
  const text=String(value??'').trim();if(!text)return [];
  try{const parsed=JSON.parse(text);return parsed===null||typeof parsed==='object'?scopeGroupIds(parsed):[String(parsed).trim()].filter(Boolean);}catch{return scopeGroupIds(text.split(/[,|;]+/));}
}
export function resolveDramaScope(meta, groups, available) {
  if(!available || !meta || (meta.app_groups===undefined && meta.appGroups===undefined) || meta.app_groups_available===false) return {status:'unresolved',names:[],unknownIds:[],reason:'分组元数据未成功获取'};
  const ids=scopeGroupIds(meta?.app_groups ?? meta?.appGroups ?? []);
  const index=new Map((groups||[]).map(g=>[String(g.id??g._id??''),String(g.name??'').trim()]));
  const names=[],unknownIds=[];
  for(const id of ids){const name=index.get(id);if(name)names.push(name);else unknownIds.push(id);}
  const allowed=names.some(name=>INSPECTION_GROUP_NAMES.some(n=>scopeGroupKey(n)===scopeGroupKey(name)));
  return {status:allowed?'included':unknownIds.length?'unresolved':'excluded',names,unknownIds,reason:unknownIds.length?'存在无法解析的分组ID':''};
}
export function filterDramaInspectionScope(ids, metadata, groups, available) {
  const includedIds=[],excludedIds=[],unresolved=[],ignoredMemberships=[];
  for(const id of [...new Set(ids)]) {
    const scope=resolveDramaScope(metadata?.[id],groups,available);
    if(scope.status==='included'){
      includedIds.push(id);
      const checkedNames=inspectionGroupNames(scope.names);
      const ignoredNames=scope.names.filter(name=>!checkedNames.includes(name));
      if(ignoredNames.length)ignoredMemberships.push({id,originalNames:scope.names,checkedNames,ignoredNames});
    }
    else if(scope.status==='excluded')excludedIds.push(id);
    else unresolved.push({id,reason:scope.reason,unknownIds:scope.unknownIds});
  }
  return {version:DRAMA_SCOPE_VERSION,allowedGroups:INSPECTION_GROUP_NAMES,sourceCount:new Set(ids).size,includedIds,excludedIds,unresolved,ignoredMemberships};
}
