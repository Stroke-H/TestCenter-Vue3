import {test} from 'node:test';import assert from 'node:assert/strict';
import {checkDramaGroups as group,checkDramaDubbing as dub,checkDramaFreeChapters as free,checkDramaCopyright as copyright,parseCopyrightExpiry as parse} from '../drama_business_rules.js';
import {INSPECTION_GROUP_NAMES,resolveDramaScope as scope,filterDramaInspectionScope as filter} from '../drama_scope.js';
const meta=(x={})=>({cnName:'普通剧',createdAt:'2026-06-01 00:00:00',unlockType:'coin',lang:'id',marketingPosition:'',freeChapters:2,freeChaptersAvailable:true,copyright:'自有',copyrightExpireAt:'2099-01-01 00:00:00',...x});
test('fifteen-group allowlist: mixed memberships preserved, unknown-only unresolved, outside-only excluded',()=>{
 assert.equal(INSPECTION_GROUP_NAMES.length,15);const gs=[{id:'a',name:'IAA 组'},{id:'x',name:'AIGC组'}];
 assert.deepEqual(scope({app_groups:['a','x']},gs,true).names,['IAA 组','AIGC组']);assert.equal(scope({app_groups:['a','x']},gs,true).status,'included');
 assert.equal(scope({app_groups:['x']},gs,true).status,'excluded');assert.equal(scope({app_groups:[]},gs,true).status,'excluded');assert.equal(scope({app_groups:['bad']},gs,true).status,'unresolved');assert.equal(scope({app_groups:['a','bad']},gs,true).status,'included');assert.equal(scope({app_groups:['a']},gs,false).status,'unresolved');
 assert.equal(scope(undefined,gs,true).status,'unresolved');assert.equal(scope({},gs,true).status,'unresolved');
 const s=filter(['1','2','1','3'],{'1':{app_groups:['a']},'2':{app_groups:['x']},'3':{app_groups:['bad']}},gs,true);assert.deepEqual(s.includedIds,['1']);assert.deepEqual(s.excludedIds,['2']);assert.equal(s.unresolved.length,1);
});
test('all fifteen nonempty coin subsets; non-whitelist extras ignored but other whitelist groups forbidden',()=>{
 const gs=['ShortsWave 组','小程序组','mangogo 组','CoCoShort 产品组'];for(let n=0;n<16;n++)assert.equal(group(meta(),gs.filter((_,i)=>n&(1<<i)),true,true).length===0,n!==0);
 assert.equal(group(meta(),['ShortsWave 组','ReelsDrama组'],true,true).length,0);
 for(const gs of [['ShortsWave 组','IAA组'],['TT_IAP_媒资库组']])assert.ok(group(meta(),gs,true,true).length);
});
test('ad: exclusive Minis OR seven nonempty traditional group subsets',()=>{
 const m=meta({unlockType:'ad'}),gs=['IAA 组','漫剧产品组 - IAA','网赚短剧'];assert.equal(group(m,['TT-Minis 分销组'],true,true).length,0);
 for(let n=0;n<8;n++)assert.equal(group(m,gs.filter((_,i)=>n&(1<<i)),true,true).length===0,n!==0);
 assert.equal(group(m,['IAA组','AIGC组'],true,true).length,0);
 for(const gs of [['TT-Minis 分销组','IAA组'],['IAA组','ShortsWave 组'],['AIGC组'],['TT_IAA_媒资库组']])assert.ok(group(m,gs,true,true).length);
});
test('38251 example and exclusivity ignore only known non-whitelist memberships',()=>{
 const m=meta({unlockType:'ad',cnName:'AI-室友契约-AIGC202605210074-AI西配(IAA)'});
 assert.equal(group(m,['IAA 组','漫剧产品组-IAA','网赚短剧','外部联运组'],true,true).length,0);
 assert.equal(group(m,['TT-Minis 分销组','外部联运组'],true,true).length,0);
 assert.ok(group(m,['TT-Minis 分销组','IAA 组','外部联运组'],true,true).length);
 for(const [x,target]of [[{cnName:'剧-DS媒资库',unlockType:'ad'},'TT_IAA_分销_媒资库'],[{cnName:'剧(媒资库)',unlockType:'coin'},'TT_IAP_媒资库组'],[{cnName:'剧媒资库(IAA)',unlockType:'ad'},'TT_IAA_媒资库组'],[{copyright:'番茄内供',unlockType:'ad'},'ShortsWave 组']]) {
  assert.equal(group(meta(x),[target,'外部联运组'],true,true).length,0);
  const other=target==='IAA组'?'ShortsWave 组':'IAA组';assert.ok(group(meta(x),[target,other,'外部联运组'],true,true).length);
 }
 assert.ok(group(m,['IAA组'],false,true).some(e=>e.type==='metadata'));
 const s=filter(['one'],{one:{app_groups:['a','outside']}},[{id:'a',name:'IAA 组'},{id:'outside',name:'外部联运组'}],true);
 assert.equal(s.ignoredMemberships.length,1);assert.deepEqual(s.ignoredMemberships[0].checkedNames,['IAA 组']);assert.deepEqual(s.ignoredMemberships[0].originalNames,['IAA 组','外部联运组']);
});
test('DS media specificity and exclusive special targets',()=>{
 for(const [cnName,target]of [['剧-DS媒资库（东南亚）','TT_IAA_分销_媒资库（香港主体）'],['剧-DS媒资库','TT_IAA_分销_媒资库'],['剧(IAA)(媒资库)(东南亚)','TT_IAA_媒资库组（香港主体）'],['剧(媒资库)(美欧)','TT_IAA_媒资库组'],['剧互动','海外互动短剧'],['剧(出海)','点石出海短剧']]){const m=meta({unlockType:'ad',cnName});assert.equal(group(m,[target],true,true).length,0,cnName);assert.ok(group(m,[target,'IAA组'],true,true).length);}
 assert.ok(group(meta({unlockType:'ad',cnName:'剧-DS媒资库(东南亚)'}),['TT_IAA_媒资库组（香港主体）'],true,true).length);
 assert.equal(group(meta({unlockType:'ad',cnName:'剧(IAA)(媒资库)'}),['TT_IAA_媒资库组'],true,true).length,0);
 assert.ok(group(meta({unlockType:'ad',cnName:'剧(出海)互动'}),['点石出海短剧'],true,true).some(e=>e.msg.includes('冲突')));
});
test('ad + media + parenthesized IAA exclusively targets TT_IAA media without overriding specific or copyright rules',()=>{
 for(const cnName of ['剧(媒资库)（IAA）','剧（媒资库）(IAA)','剧媒资库(iaa)']) {
  const m=meta({unlockType:'ad',cnName,createdAt:'2020-01-01'});
  assert.equal(group(m,['TT_IAA_媒资库组'],true,true).length,0);
  for(const gs of [['IAA组'],['TT-Minis 分销组'],['TT_IAA_媒资库组','IAA组'],[]])assert.ok(group(m,gs,true,true).length);
 }
 assert.equal(group(meta({unlockType:'coin',cnName:'剧(媒资库)(IAA)'}),['ShortsWave 组'],true,true).length,0);
 assert.ok(group(meta({unlockType:'coin',cnName:'剧(媒资库)(IAA)'}),['TT_IAA_媒资库组'],true,true).length);
 assert.ok(group(meta({unlockType:'ad',cnName:'剧(IAA)'}),['TT_IAA_媒资库组'],true,true).length);
 for(const [cnName,target]of [['剧-DS媒资库(IAA)','TT_IAA_分销_媒资库'],['剧-DS媒资库(东南亚)(IAA)','TT_IAA_分销_媒资库（香港主体）'],['剧(媒资库)(东南亚)(IAA)','TT_IAA_媒资库组（香港主体）']])assert.equal(group(meta({unlockType:'ad',cnName}),[target],true,true).length,0);
 assert.equal(group(meta({unlockType:'ad',cnName:'剧(媒资库)(IAA)',copyright:'番茄内供'}),['ShortsWave 组'],true,true).length,0);
});
test('coin exact media suffix; tomato copyright overrides all special group rules',()=>{
 assert.equal(group(meta({cnName:'剧(媒资库)'}),['TT_IAP_媒资库组'],true,true).length,0);assert.ok(group(meta({cnName:'剧(媒资库)其他'}),['TT_IAP_媒资库组'],true,true).length);
 const m=meta({unlockType:'ad',cnName:'(出海)互动-DS媒资库(东南亚)',copyright:'番茄内供'});assert.equal(group(m,['ShortsWave 组'],true,true).length,0);for(const gs of [['点石出海短剧'],['ShortsWave 组','IAA组']])assert.ok(group(m,gs,true,true).length);
});
test('overseas special group requires an exact parenthesized marker, never bare title words',()=>{
 for(const cnName of ['剧(出海)','剧（出海）','(出海)剧','剧(出海)(IAA)']) {
  const m=meta({unlockType:'ad',cnName});
  assert.equal(group(m,['点石出海短剧'],true).length,0,cnName);
  for(const names of [['IAA组'],['点石出海短剧','IAA组'],[]])assert.ok(group(m,names,true).length,cnName);
 }
 for(const cnName of ['出海','一起出海去','剧(出海版)','剧（出海短剧）','剧出海）','剧（出海','剧(出海计划)']) {
  const m=meta({unlockType:'ad',cnName});
  assert.equal(group(m,['IAA组'],true).length,0,cnName);
  assert.equal(group(m,['TT-Minis 分销组'],true).length,0,cnName);
  assert.ok(group(m,['点石出海短剧'],true).length,cnName);
 }
 assert.equal(group(meta({unlockType:'coin',cnName:'剧（出海）'}),['ShortsWave组'],true).length,0);
 assert.equal(group(meta({unlockType:'ad',cnName:'剧（出海）',copyright:'番茄内供'}),['ShortsWave组'],true).length,0);
 assert.ok(group(meta({unlockType:'ad',cnName:'剧（出海）（IAA）（媒资库）'}),['点石出海短剧'],true).some(e=>e.msg.includes('冲突')));
});
test('ad IAA media plus Hong Kong requires the exclusive Hong Kong group, preserving higher priorities',()=>{
 const hk='TT_IAA_媒资库组（香港主体）', ordinary='TT_IAA_媒资库组';
 for(const cnName of ['剧(IAA)(媒资库)（香港主体）','剧（iaa）（媒资库）(香港主体)','剧(香港主体)(媒资库)(IAA)','剧(IAA)(媒资库)(香港主体)(美欧)']) {
  const m=meta({unlockType:'ad',cnName});
  assert.equal(group(m,[hk],true).length,0,cnName);
  assert.equal(group(m,[hk,'外部联运组'],true).length,0);
  for(const names of [[ordinary],[hk,ordinary],['IAA组'],[]])assert.ok(group(m,names,true).length,cnName);
 }
 // Missing or partial tokens must not accidentally activate the new branch.
 for(const cnName of ['剧(IAA)(媒资库)','剧(IAA)(媒资库)(香港主体其他)','剧(IAA)媒资库(香港主体)'])assert.equal(group(meta({unlockType:'ad',cnName}),[ordinary],true).length,0,cnName);
 assert.equal(group(meta({unlockType:'coin',cnName:'剧(IAA)(媒资库)(香港主体)'}),['ShortsWave组'],true).length,0);
 assert.equal(group(meta({unlockType:'ad',copyright:'番茄内供',cnName:'剧(IAA)(媒资库)(香港主体)'}),['ShortsWave组'],true).length,0);
 assert.equal(group(meta({unlockType:'ad',cnName:'剧-DS媒资库(IAA)(媒资库)(香港主体)'}),['TT_IAA_分销_媒资库'],true).length,0);
});
test('unknown unlock and missing group metadata fail closed',()=>{for(const unlockType of ['','coins','bad'])assert.ok(group(meta({unlockType}),['IAA组'],true,true).length);assert.ok(group(meta(),[],false,true).some(e=>e.type==='metadata'))});
test('X配 / X全剧配音 require language AND tag, including content-five and tomato; no reverse check',()=>{
 for(const cnName of ['剧(AI印配)','剧(印全剧配音)','内容五部(印尼全剧配音)','番茄剧(印配)']){assert.equal(dub(meta({cnName,marketingPosition:'配音剧'})).length,0);assert.equal(dub(meta({cnName,lang:'pt'})).length,2);}
 assert.equal(dub(meta({marketingPosition:'配音剧'})).length,0);
 for(const [marker,lang]of [['西','es'],['葡','pt-BR'],['日','ja'],['韩','ko'],['阿','ar-SA'],['繁体中文','zh-TW']])assert.equal(dub(meta({cnName:`剧(${marker}全剧配音)`,lang,marketingPosition:'都市,配音剧'})).length,0);
 assert.ok(dub(meta({cnName:'剧(英配)(葡配)',lang:'en',marketingPosition:'配音剧'})).some(e=>e.msg.includes('不同语种')));assert.ok(dub(meta({cnName:'剧(越全剧配音)'})).some(e=>e.msg.includes('未配置语种映射')));
});
test('ad free_chapters=2, no content-five exemption',()=>{for(const v of [2,'2'])assert.equal(free(meta({unlockType:'ad',freeChapters:v})).length,0);for(const v of [0,3,-1,null,undefined,'',true,'2x'])assert.ok(free(meta({unlockType:'ad',freeChapters:v})).length);assert.ok(free(meta({cnName:'内容五部',unlockType:'ad',freeChapters:3})).length);assert.equal(free(meta({freeChapters:3})).length,0)});
test('only dubbing checks use created_at cutoff; malformed or missing dates skip rather than pass a classification',()=>{
 for(const createdAt of ['2026-05-31 23:59:59','2026-05-31T15:59:59Z',null,undefined,'','bad','2026-02-30 00:00:00'])assert.equal(dub(meta({createdAt,cnName:'剧(葡全剧配音)',lang:'id'})).length,0,String(createdAt));
 for(const createdAt of ['2026-06-01','2026-06-01 00:00:00','2026-05-31T16:00:00Z',Date.parse('2026-06-01T00:00:00+08:00'),Date.parse('2026-06-01T00:00:00+08:00')/1000])assert.equal(dub(meta({createdAt,cnName:'剧(葡配)',lang:'id'})).length,2,String(createdAt));
 assert.equal(dub({cnName:'剧(葡配)',created_at:'2026-06-01',lang:'id'}).length,2);
 const old=meta({createdAt:null,unlockType:'ad',freeChapters:3,copyrightExpireAt:'2000-01-01'});assert.ok(group(old,['ShortsWave 组'],true,true).length);assert.ok(free(old).length);assert.ok(copyright(old).length);
});
test('copyright strict future boundary, explicit timezone, missing/invalid/calendar errors',()=>{
 const now=Date.parse('2026-11-05T10:00:00+08:00');assert.equal(parse('2026-11-05 10:00:00'),now);
 for(const expiry of ['2026-11-05 10:00:01','2026-11-05T02:00:01Z',now+1,(now+1000)/1000])assert.equal(copyright(meta({copyrightExpireAt:expiry}),now).length,0);
 for(const expiry of ['2026-11-05 10:00:00','2020-01-01',null,'','bad','2026-02-30 00:00:00','2026-11-05 25:00:00',{},0])assert.ok(copyright(meta({copyrightExpireAt:expiry}),now).length,JSON.stringify(expiry));
});
