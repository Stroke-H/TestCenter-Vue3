const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const base=path.join(__dirname,'..');const source=['drama_scope.js','drama_business_rules.js','drama_prepared_rules.js','drama_check_flow.js'].map(n=>fs.readFileSync(path.join(base,n),'utf8').replace(/^import .*;\r?$/gm,'').replace('export default function ()','function runDramaCase()').replace(/export (function|const) /g,'$1 ')).join('\n');
function runtime({retry=false,override={},language='passed',config={},outOfScope=false,empty=false,metadataRetry=false,failHTTP=false,afterPrepare}={}){
 override={created_at:'2026-06-01 00:00:00',...override};
 const id='111111111111111111111111',checks=[];let requests=0;
 const sandbox={console:{log(){},warn(){},error(){}},http:{get(){requests++;return{status:200,body:JSON.stringify({total:0,data:[]})}}},check:(_,v)=>checks.push(v),sleep(){},Counter:class{add(){}},SharedArray:function(_,f){return f()},__ENV:{DRAMA_RETRY_PHASE:retry?'1':'0'},__VU:1,__ITER:0,
 open:n=>{
  if(n.includes('k6_config'))return JSON.stringify(config);
  const payload={dramaList:empty?[]:[id],inspectionScope:{version:'2026-11-05-v4',sourceCount:2,excludedIds:['excluded'],includedIds:empty||outOfScope?[]:[id]},auth:{x_token:''},appGroups:[{id:'g',name:outOfScope?'AIGC组':'ShortsWave 组'}],validationMeta:{appGroups:{available:true}},descriptionLanguageValidation:{ready:true,counts:{}},dramaMeta:{[id]:{cn_name:'普通剧',app_groups:['g'],unlock_type:'coin',lang:'en',description:'English text',description_language_check:{status:language,detected:'ja',description_hash:'hash'},free_chapters:2,free_chapters_available:true,copyright:'自有',copyright_expire_at:'2099-01-01 00:00:00',...override}}};
  const {failures,candidates,...validation}=sandbox.prepareDramaRuleChecks(payload.dramaList,payload.dramaMeta,payload.appGroups,true,config,retry?'retry':'initial',Date.now(),{metadataRetryIds:metadataRetry?[id]:[]});
  payload.preparedRuleValidation=validation;
  if(afterPrepare)afterPrepare(payload);
  return JSON.stringify(payload);
 }};
 if(metadataRetry){sandbox.__ENV.DRAMA_METADATA_RETRY_IDS_FILE='metadata-retry-ids';const load=sandbox.open;sandbox.open=n=>n==='metadata-retry-ids'?JSON.stringify([id]):load(n);}
 if(failHTTP)sandbox.http.get=()=>{requests++;return{status:401,body:'{}'}};
 vm.createContext(sandbox);vm.runInContext(source,sandbox);vm.runInContext('runDramaCase()',sandbox);return{sandbox,checks,requests};
}
const names=r=>r.checks.flatMap(Object.keys).filter(n=>!n.startsWith('__DRAMA_COMPLETED__|')).map(decodeURIComponent).join('\n');
test('outside-allowlist input is rejected before playback',()=>assert.throws(()=>runtime({outOfScope:true}),/白名单范围/));
test('unresolved group IDs are not silently assumed to be ignorable non-whitelist groups',()=>{for(const retry of [false,true]){const r=runtime({retry,override:{app_groups:['g','unresolved-id']}});assert.equal(r.requests,1);assert.match(names(r),/App Group元数据/);}});
test('no eligible dramas produces an empty-scope report without HTTP or reporter metrics',()=>{const r=runtime({empty:true});assert.equal(r.requests,0);r.sandbox.summary={root_group:{checks:[],groups:[]},metrics:{}};const o=vm.runInContext('handleSummary(summary)',r.sandbox);assert.equal(JSON.parse(o[Object.keys(o).find(k=>k.endsWith('drama_initial_completion.json'))]).expectedIds.length,0);assert.ok(Object.values(o).some(v=>v.includes('无白名单范围')));});
test('content-five has no exemption; explicit data findings occur initially, class/language confirmed on retry',()=>{for(const retry of [false,true]){const r=runtime({retry,language:'mismatch',override:{cn_name:'内容五部(AI印全剧配音)',unlock_type:'ad',free_chapters:3,copyright_expire_at:'2000-01-01'}});assert.equal(r.requests,1);for(const label of ['简介语种','分类规则'])assert.ok(names(r).includes(label),label);for(const label of ['分组规则','版权规则','免费集数'])assert.equal(names(r).includes(label),!retry,label);}});
test('date cutoff skips only classification in initial and retry, not description or other modules',()=>{
 for(const retry of [false,true])for(const created_at of ['2026-05-31 23:59:59',null,'bad']){
  const r=runtime({retry,language:'mismatch',override:{created_at,cn_name:'剧(葡全剧配音)',lang:'id',unlock_type:'ad',free_chapters:3,copyright_expire_at:'2000-01-01'}});
  assert.equal(r.requests,1);assert.ok(!names(r).includes('分类规则'));assert.ok(names(r).includes('简介语种'));for(const label of ['分组规则','免费集数','版权规则'])assert.equal(names(r).includes(label),!retry,label);
 }
});
function summary(r){r.sandbox.htmlReport=()=>'<html><body><section class="drama-other-checks"></section></body></html>';r.sandbox.summary={root_group:{checks:r.checks.flatMap(c=>Object.entries(c).map(([name,pass])=>({name,passes:pass?1:0,fails:pass?0:1}))),groups:[]},metrics:{}};return vm.runInContext('handleSummary(summary)',r.sandbox);}
function jsonOutput(output,suffix){return JSON.parse(output[Object.keys(output).find(k=>k.endsWith(suffix))]);}
test('explicit group/unlock/free/copyright errors are reported initially without creating retry candidates',()=>{
 for(const [override,label]of [[{unlock_type:'ad'},'分组规则'],[{unlock_type:''},'解锁类型'],[{unlock_type:'coins'},'解锁类型'],[{unlock_type:'ad',copyright:'番茄内供',free_chapters:3},'免费集数'],[{copyright_expire_at:null},'版权规则'],[{copyright_expire_at:'2000-01-01'},'版权规则']]){
  const first=runtime({override}),o=summary(first);assert.equal(jsonOutput(o,'drama_retry_candidates.json').candidates.length,0,label);assert.ok(jsonOutput(o,'drama_failure_summary.json').failures.some(f=>f.errors.some(e=>e.includes('['+label+']'))),label);
  assert.ok(!names(runtime({retry:true,override})).includes('['+label+']'),'must not recheck explicit data on unrelated retry');
 }
});
test('mixed findings: initial direct errors retained; only classification and language go into retry',()=>{
 const options={language:'mismatch',override:{cn_name:'剧(葡配)',unlock_type:'ad',free_chapters:3,copyright_expire_at:'2000-01-01'}};
 const first=summary(runtime(options)),c=jsonOutput(first,'drama_retry_candidates.json').candidates;assert.equal(c.length,1);assert.ok(c[0].kind.includes('dubbing_classification'));assert.ok(c[0].kind.includes('description_language'));assert.ok(!/分组规则|免费集数|版权规则|解锁类型/.test(c[0].reason));
 const before=jsonOutput(first,'drama_failure_summary.json').failures[0].errors.join('\n');assert.match(before,/分组规则/);assert.match(before,/免费集数/);assert.match(before,/版权规则/);assert.ok(!/分类规则|简介语种/.test(before));
 const after=jsonOutput(summary(runtime({...options,retry:true})),'drama_retry_result.json').structuredFailures[0].errors.join('\n');assert.match(after,/分类规则/);assert.match(after,/简介语种/);assert.ok(!/分组规则|免费集数|版权规则|解锁类型/.test(after));
});
test('720 fetch failures still retry without losing direct findings',()=>{
 const o=summary(runtime({failHTTP:true,override:{copyright_expire_at:null}}));const c=jsonOutput(o,'drama_retry_candidates.json').candidates;assert.equal(c.length,1);assert.equal(c[0].kind,'720_fetch');assert.ok(!c[0].reason.includes('版权规则'));assert.ok(jsonOutput(o,'drama_failure_summary.json').failures[0].errors.some(e=>e.includes('版权规则')));
});
test('resolved metadata gets its first group evaluation, not repeated free/copyright evaluation',()=>{
 const r=runtime({retry:true,metadataRetry:true,override:{unlock_type:'ad',free_chapters:3,copyright_expire_at:null}});assert.match(names(r),/分组规则/);assert.ok(!/免费集数|版权规则/.test(names(r)));
});
test('switches remain reversible without disabling playback, description or dubbing',()=>{const r=runtime({language:'mismatch',config:{skipGroupChecks:true,skipUnlockTypeChecks:true,skipFreeChapterChecks:true,skipCopyrightChecks:true},override:{cn_name:'剧(出海)(AI印配)',unlock_type:'unknown',free_chapters:8,copyright_expire_at:null}});assert.equal(r.requests,1);assert.match(names(r),/分类规则/);assert.match(names(r),/简介语种/);assert.ok(!/分组规则|解锁类型|版权规则|免费集数/.test(names(r)));});
test('uncertain, empty, passed descriptions do not raise language findings',()=>{for(const language of ['passed','skipped','uncertain','mixed','unsupported'])assert.ok(!names(runtime({language})).includes('简介语种'))});
test('actual completion and rule/filter snapshots in both phases, not planned iterations',()=>{for(const retry of [false,true]){
 const r=runtime({retry});r.sandbox.htmlReport=()=>'<html><body><section class="drama-other-checks"></section></body></html>';r.sandbox.summary={root_group:{checks:[{name:'__DRAMA_COMPLETED__|111111111111111111111111',passes:1,fails:0}],groups:[]},metrics:{checks:{values:{passes:1,fails:0}},iterations:{values:{count:100}}}};
 const o=vm.runInContext('handleSummary(summary)',r.sandbox),inventory=JSON.parse(o[Object.keys(o).find(k=>k.endsWith(retry?'drama_retry_completion.json':'drama_initial_completion.json'))]);assert.equal(inventory.completedIds.length,1);assert.equal(inventory.missingIds.length,0);
 const p=JSON.parse(o[Object.keys(o).find(k=>k.endsWith(retry?'drama_retry_result.json':'drama_failure_summary.json'))]);for(const k of ['groupChecks','unlockTypeChecks','freeChapterChecks','copyrightChecks','languageChecks','dubbingClassificationChecks','playbackChecks'])assert.equal(p.ruleScope[k],true,k);assert.equal(p.ruleScope.excludedCount,1);assert.equal(p.ruleScope.contentFive,undefined);assert.equal(r.sandbox.summary.root_group.checks.length,0);
}});
test('all configured checks reenabled',()=>{const c=JSON.parse(fs.readFileSync(path.join(base,'k6_config.json')));for(const k of ['skipGroupChecks','skipUnlockTypeChecks','skipFreeChapterChecks','skipCopyrightChecks','skipDubbingClassificationChecks'])assert.equal(c[k],false,k)});
test('manual language review evidence is archived separately for lazy loading, never counted as a failure',()=>{
 for(const retry of [false,true]) {
  const output=summary(runtime({retry,language:'uncertain'}));
  const review=jsonOutput(output,retry?'drama_retry_language_review.json':'drama_initial_language_review.json');
  assert.equal(review.rows.length,1);assert.equal(review.rows[0].description,'English text');assert.equal(review.rows[0].status,'uncertain');
  assert.equal(review.checkedIds.length,retry?1:0);assert.ok(!JSON.stringify(review).includes('x_token'));
  assert.equal(jsonOutput(output,retry?'drama_retry_result.json':'drama_failure_summary.json')[retry?'structuredFailures':'failures'].length,0);
 }
});
test('k6 consumes collection findings instead of re-evaluating mutable raw metadata',()=>{
 const r=runtime({override:{unlock_type:'ad',free_chapters:3,copyright_expire_at:null,cn_name:'剧(葡配)'},afterPrepare:p=>Object.assign(Object.values(p.dramaMeta)[0],{unlock_type:'coin',free_chapters:2,copyright_expire_at:'2099-01-01',marketing_position:'配音剧',lang:'pt'})});
 const o=summary(r),errors=jsonOutput(o,'drama_failure_summary.json').failures[0].errors.join('\n');
 for(const label of ['分组规则','免费集数','版权规则'])assert.match(errors,new RegExp(label));
 assert.match(jsonOutput(o,'drama_retry_candidates.json').candidates[0].reason,/分类规则/);
 const raw=vm.runInContext(`getRawDramaMeta('111111111111111111111111')`,r.sandbox);
 for(const key of ['app_groups','copyright','free_chapters','unlock_type','created_at'])assert.equal(raw[key],undefined,key);
 assert.equal(raw.description,'','successful descriptions need not be copied into every case');
 assert.equal(vm.runInContext('config.dramaMeta',r.sandbox),undefined);
 assert.equal(vm.runInContext('data.length',r.sandbox),2,'one header plus individually shared records');
 const flow=fs.readFileSync(path.join(base,'drama_check_flow.js'),'utf8');assert.ok(!/checkDrama(?:Groups|Dubbing|FreeChapters|Copyright)\(/.test(flow));assert.ok(!flow.includes('resolveDramaScope('));
});
test('missing, stale, partial or mismatched collection snapshots fail closed before playback',()=>{
 for(const afterPrepare of [p=>delete p.preparedRuleValidation,p=>p.preparedRuleValidation.version='old',p=>p.preparedRuleValidation.phase='retry',p=>p.preparedRuleValidation.checkedCount=99,p=>p.preparedRuleValidation.settings.skipGroupChecks=true,p=>delete Object.values(p.dramaMeta)[0].prepared_rules,p=>delete Object.values(p.dramaMeta)[0].prepared_rules.directErrors])assert.throws(()=>runtime({afterPrepare}),/字段规则|规则开关/);
 assert.throws(()=>runtime({afterPrepare:p=>delete Object.values(p.dramaMeta)[0].description_language_check}),/简介语种检测结果/);
});
test('copyright fields and fresh empty group lists override stale metadata; CSV preserves rights fields',()=>{
 const p=fs.readFileSync(path.join(base,'prepare_data.js'),'utf8').replace(/^import .*;\r?$/gm,'').replace(/main\(\)\.catch\(err => \{[\s\S]*?\r?\n\}\);/,'').replace(/import\.meta\.dirname/g,JSON.stringify(base));
 const ctx={fs,path,process:{env:{}},console:{log(){},warn(){},error(){}}};vm.createContext(ctx);vm.runInContext(p,ctx);
 const v=vm.runInContext(`mergeDramaMeta({app_groups:['old'],copyright:'旧版权',copyright_expire_at:'2099-01-01'},{app_groups:[],copyright:'番茄内供',copyright_expire_at:null})`,ctx);assert.equal(v.copyright,'番茄内供');assert.equal(v.copyright_expire_at,null);assert.equal(v.app_groups.length,0);assert.equal(v.app_groups_available,true);assert.equal(v.copyright_expire_at_available,true);
 const fresh=vm.runInContext(`mergeDramaMeta({unlock_type:'coin',marketing_position:'配音剧',cn_name:'旧名称'},{unlock_type:'',marketing_position:'',cn_name:'新名称'})`,ctx);assert.equal(fresh.unlock_type,'');assert.equal(fresh.marketing_position,'');assert.equal(fresh.cn_name,'新名称');
 const csv=vm.runInContext(`buildDramaMetaFromRecord(['_id','copyright','copyright_expire_at','app_groups'],['id','番茄内供','2027-01-01 00:00:00','["g"]'])`,ctx);assert.equal(csv.copyright,'番茄内供');assert.equal(csv.copyright_expire_at,'2027-01-01 00:00:00');assert.equal(csv.app_groups[0],'g');
});
