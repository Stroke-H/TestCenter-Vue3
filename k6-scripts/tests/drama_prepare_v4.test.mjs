import {test} from 'node:test';import assert from 'node:assert/strict';import fs from 'node:fs';import os from 'node:os';import path from 'node:path';import http from 'node:http';import {execFile} from 'node:child_process';import {promisify} from 'node:util';
const exec=promisify(execFile);const source=path.resolve(import.meta.dirname,'..');
for(const format of ['csv','json','unresolved'])test(`actual preparation pipeline: ${format}, filter precedes further checking`,async t=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'testcenter-scope-v4-'));t.after(()=>fs.rmSync(root,{recursive:true,force:true}));fs.mkdirSync(path.join(root,'k6-scripts'));
 for(const file of ['prepare_data.js','drama_scope.js','drama_business_rules.js','drama_prepared_rules.js'])fs.copyFileSync(path.join(source,file),path.join(root,'k6-scripts',file));
 fs.writeFileSync(path.join(root,'package.json'),JSON.stringify({type:'module'}));
 const ids=[1,2,3,4].map(n=>n.toString(16).padStart(24,'0'));
 const played=[];
 const rows=ids.map((id,i)=>({id,int_id:i+1,cn_name:i===3?'内容五部(印全剧配音)':'剧'+i,app_groups:i===1?['outside']:i===2?['allowed','outside']:[format==='unresolved'?'missing':'allowed'],unlock_type:'coin',description:'',lang:'id',copyright:'番茄内供',copyright_expire_at:i===3?'2000-01-01 00:00:00':'2099-01-01 00:00:00'}));
 for(const row of rows)row.created_at='2026-06-01 00:00:00';
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json');
  if(req.url==='/api/pwd_login'){res.setHeader('Set-Cookie','x-token=fake-local-test-token; Path=/');res.end('{}');}
  else if(req.url.startsWith('/api/management/drama/all_online_ids'))res.end(format==='csv'?'_id,int_id\n'+ids.map((id,i)=>id+','+i).join('\n'):JSON.stringify(rows));
  else if(req.url.startsWith('/api/management/drama/list?'))res.end(JSON.stringify({data:rows,total:rows.length}));
  else if(req.url.startsWith('/api/management/app/group/list'))res.end(JSON.stringify({data:[{id:'allowed',name:'ShortsWave 组'},{id:'outside',name:'AIGC组'}]}));
  else if(req.url.startsWith('/api/management/drama/chapter/list?')){played.push(new URL(req.url,'http://localhost').searchParams.get('drama_id'));res.end(JSON.stringify({total:0,data:[]}));}
  else{res.statusCode=404;res.end('{}');}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));t.after(()=>new Promise(resolve=>server.close(resolve)));
 const base='http://127.0.0.1:'+server.address().port;
 const env={...process.env,EMAIL:'local-test',PASSWORD:'local-test',LOGIN_URL:base+'/api/pwd_login',DRAMA_LIST_URL:base+'/api/management/drama/all_online_ids',APP_GROUP_LIST_URL:base+'/api/management/app/group/list',DRAMA_REPORT_DIR:'report',PREPARE_FETCH_RETRY_ATTEMPTS:'1',PREPARE_FETCH_TIMEOUT_MS:'3000'};
 if(format==='unresolved'){
  await assert.rejects(exec(process.execPath,[path.join(root,'k6-scripts','prepare_data.js')],{cwd:root,env}),/白名单范围无法确认/);
  assert.ok(!fs.existsSync(path.join(root,'drama_info.json')));
  const s=JSON.parse(fs.readFileSync(path.join(root,'report','drama_scope.json')));assert.ok(s.unresolved.length>0);return;
 }
 await exec(process.execPath,[path.join(root,'k6-scripts','prepare_data.js')],{cwd:root,env});
 const result=JSON.parse(fs.readFileSync(path.join(root,'drama_info.json')));
 assert.deepEqual(result.dramaList,[ids[0],ids[2],ids[3]]);assert.equal(result.dramaMeta[ids[1]],undefined);assert.deepEqual(result.dramaMeta[ids[2]].app_groups,['allowed','outside']);
 assert.equal(result.dramaMeta[ids[0]].copyright,'番茄内供');assert.equal(result.dramaMeta[ids[0]].copyright_expire_at,'2099-01-01 00:00:00');assert.equal(result.inspectionScope.excludedIds.length,1);assert.ok(fs.existsSync(path.join(root,'report','drama_initial_scope.json')));
 assert.equal(result.preparedRuleValidation.ready,true);assert.equal(result.preparedRuleValidation.checkedCount,3);
 const rules=JSON.parse(fs.readFileSync(path.join(root,'report','drama_initial_rules.json')));
 assert.ok(rules.failures.some(item=>item.dramaId===ids[3]&&item.errors.some(error=>error.includes('[版权规则]'))));
 assert.ok(rules.candidates.some(item=>item.dramaId===ids[3]&&item.kind==='dubbing_classification'));
 assert.ok(!rules.failures.some(item=>item.dramaId===ids[1]));assert.equal(played.length,0,'metadata rules finish before any playback request');
 if(format==='json') {
  for(const file of ['drama_check_flow.js','drama_business_rules.js'])fs.copyFileSync(path.join(source,file),path.join(root,'k6-scripts',file));
  fs.mkdirSync(path.join(root,'k6-scripts','vendor','k6-reporter'),{recursive:true});fs.copyFileSync(path.join(source,'vendor','k6-reporter','bundle.js'),path.join(root,'k6-scripts','vendor','k6-reporter','bundle.js'));
  fs.writeFileSync(path.join(root,'k6-scripts','k6_config.json'),JSON.stringify({vus:2,retryVus:2,maxDuration:'1m',timeout:'3s',retryAttempts:1}));
  for(const meta of Object.values(result.dramaMeta))meta.description_language_check={status:'skipped'};
  result.descriptionLanguageValidation={ready:true,counts:{skipped:3},total:3};fs.writeFileSync(path.join(root,'drama_info.json'),JSON.stringify(result));
  await exec('k6',['run','--quiet','-e','DRAMA_REPORT_DIR=report','k6-scripts/drama_check_flow.js'],{cwd:root,env});
  const completion=JSON.parse(fs.readFileSync(path.join(root,'report','drama_initial_completion.json')));assert.deepEqual(new Set(completion.completedIds),new Set(result.dramaList));assert.equal(completion.missingIds.length,0);assert.ok(!played.includes(ids[1]));assert.ok(played.includes(ids[3]));
  const candidates=JSON.parse(fs.readFileSync(path.join(root,'report','drama_retry_candidates.json'))).candidates;assert.equal(candidates.length,1);
  assert.equal(candidates[0].kind,'dubbing_classification');assert.ok(!candidates[0].reason.includes('[版权规则]'));
  const initialFailures=JSON.parse(fs.readFileSync(path.join(root,'report','drama_failure_summary.json'))).failures;assert.ok(initialFailures.some(f=>f.errors.some(e=>e.includes('[版权规则]'))));
  assert.equal(result.inspectionScope.ignoredMemberships.length,1);assert.deepEqual(result.inspectionScope.ignoredMemberships[0].originalNames,['ShortsWave 组','AIGC组']);
  const subset=path.join(root,'retry_ids.json');fs.writeFileSync(subset,JSON.stringify(candidates.map(c=>c.dramaId)));fs.mkdirSync(path.join(root,'retry'));
  // Real confirmation refreshes metadata and precomputes only uncertain rules.
  await exec(process.execPath,[path.join(root,'k6-scripts','prepare_data.js')],{cwd:root,env:{...env,DRAMA_PREPARE_PHASE:'retry'}});
  const refreshed=JSON.parse(fs.readFileSync(path.join(root,'drama_info.json')));
  for(const meta of Object.values(refreshed.dramaMeta))meta.description_language_check={status:'skipped'};
  refreshed.descriptionLanguageValidation={ready:true,counts:{skipped:3},total:3};fs.writeFileSync(path.join(root,'drama_info.json'),JSON.stringify(refreshed));
  const retryRules=JSON.parse(fs.readFileSync(path.join(root,'report','drama_retry_rules.json')));
  assert.equal(retryRules.failures.length,0,'explicit findings are not recomputed during confirmation');
  await exec('k6',['run','--quiet','-e','DRAMA_REPORT_DIR=retry','-e','DRAMA_RETRY_PHASE=1','-e','DRAMA_CASE_IDS_FILE='+subset,'k6-scripts/drama_check_flow.js'],{cwd:root,env});
  const retry=JSON.parse(fs.readFileSync(path.join(root,'retry','drama_retry_result.json')));assert.equal(retry.structuredFailures.length,1);assert.ok(retry.structuredFailures.some(f=>f.errors.some(e=>e.includes('[分类规则]'))));assert.ok(!retry.structuredFailures.some(f=>f.errors.some(e=>e.includes('[版权规则]'))));assert.ok(!played.includes(ids[1]));
 }
});
