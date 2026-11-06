import { test } from 'node:test';
import assert from 'node:assert/strict';
import { prepareDramaRuleChecks, ruleSettings } from '../drama_prepared_rules.js';

const groups = [{ id: 'sw', name: 'ShortsWave 组' }, { id: 'iaa', name: 'IAA组' }, { id: 'outside', name: '外部联运组' }];
const now = Date.parse('2026-11-05T12:00:00+08:00');
const meta = (changes = {}) => ({
  int_id: '1', title: 'Drama', cn_name: '普通剧', created_at: '2026-06-01 00:00:00',
  unlock_type: 'coin', app_groups: ['sw'], lang: 'en', marketing_position: '',
  free_chapters: 2, free_chapters_available: true, copyright: '自有',
  copyright_expire_at: '2099-01-01', ...changes,
});
function run(records, phase = 'initial', settings = {}, retry = {}) {
  const result = prepareDramaRuleChecks(Object.keys(records), records, groups, true, settings, phase, now, retry);
  return { result, records };
}

test('collection records direct findings and confirmation candidates separately, with one timestamp', () => {
  const { result, records } = run({ one: meta({ unlock_type: 'ad', free_chapters: 3, copyright_expire_at: '2000-01-01', cn_name: '剧(葡配)' }) });
  const r = records.one.prepared_rules;
  for (const type of ['unlock', 'free_chapters', 'copyright']) assert.ok(r.directErrors.some(e => e.type === type));
  assert.ok(r.classificationErrors.length);
  assert.equal(result.failures.length, 1);
  assert.equal(result.candidates.length, 1);
  assert.equal(result.candidates[0].kind, 'dubbing_classification');
  assert.ok(!/版权规则|免费集数|分组规则/.test(result.candidates[0].reason));
  assert.equal(result.generatedAt, new Date(now).toISOString());
  assert.deepEqual(result.settings, ruleSettings());
});

test('copyright equality is checked at collection time', () => {
  const { records } = run({ one: meta({ copyright_expire_at: now }) });
  assert.match(records.one.prepared_rules.directErrors[0].msg, /版权已过期/);
});

test('collection only applies overseas exclusivity to the complete parenthesized marker', () => {
  const gs=[...groups,{id:'overseas',name:'点石出海短剧'}];
  const records={
    title:meta({cn_name:'一起出海去',unlock_type:'ad',app_groups:['iaa']}),
    otherMarker:meta({cn_name:'剧(出海版)',unlock_type:'ad',app_groups:['iaa']}),
    marked:meta({cn_name:'剧（出海）',unlock_type:'ad',app_groups:['overseas']}),
    wrong:meta({cn_name:'剧(出海)',unlock_type:'ad',app_groups:['iaa']}),
  };
  const result=prepareDramaRuleChecks(Object.keys(records),records,gs,true,{},'initial',now);
  assert.deepEqual(result.failures.map(f=>f.dramaId),['wrong']);
  assert.match(records.wrong.prepared_rules.directErrors[0].msg,/有且只有 点石出海短剧/);
  assert.equal(result.candidates.length,0);
});

test('Hong Kong IAA media rule is evaluated during collection, ordinary IAA media remains valid', () => {
  const gs=[...groups,{id:'hk',name:'TT_IAA_媒资库组（香港主体）'},{id:'media',name:'TT_IAA_媒资库组'}];
  const records={
    hk:meta({cn_name:'AI-小黑客大佬-AI韩配(IAA)(媒资库)（香港主体）',unlock_type:'ad',app_groups:['hk']}),
    wrong:meta({cn_name:'剧(IAA)(媒资库)(香港主体)',unlock_type:'ad',app_groups:['media']}),
    ordinary:meta({cn_name:'剧(IAA)(媒资库)',unlock_type:'ad',app_groups:['media']}),
  };
  const result=prepareDramaRuleChecks(Object.keys(records),records,gs,true,{},'initial',now);
  assert.deepEqual(records.hk.prepared_rules.directErrors,[]);
  assert.deepEqual(records.ordinary.prepared_rules.directErrors,[]);
  assert.match(records.wrong.prepared_rules.directErrors[0].msg,/有且只有 TT_IAA_媒资库组（香港主体）/);
  assert.equal(result.failures.length,1);assert.equal(result.failures[0].dramaId,'wrong');
  assert.ok(!result.candidates.some(c=>c.reason.includes('[分组规则]')));
});

test('confirmation does not re-evaluate known group/unlock/free/copyright errors', () => {
  const { result, records } = run({ one: meta({ unlock_type: '', free_chapters: 99, copyright_expire_at: null, cn_name: '剧(葡配)' }), untouched: meta({ cn_name: '剧(葡配)' }) }, 'retry', {}, { candidateIds: ['one'] });
  assert.deepEqual(records.one.prepared_rules.directErrors, []);
  assert.deepEqual(records.one.prepared_rules.resolvedGroupErrors, []);
  assert.ok(records.one.prepared_rules.classificationErrors.length);
  assert.deepEqual(records.untouched.prepared_rules.classificationErrors, []);
  assert.deepEqual(result.failures, []);
});

test('uncertain metadata is confirmed; only previously unresolved IDs get their first group decision', () => {
  const unknown = run({ one: meta({ app_groups: ['sw', 'unknown'] }) });
  assert.equal(unknown.result.candidates[0].kind, 'app_group_metadata_case');
  const retry = run({ one: meta({ unlock_type: 'ad', free_chapters: 8, copyright_expire_at: null }), two: meta({ unlock_type: 'ad' }) }, 'retry', {}, { candidateIds: ['one', 'two'], metadataRetryIds: ['one'] });
  assert.match(retry.records.one.prepared_rules.resolvedGroupErrors[0].msg, /分组规则/);
  assert.deepEqual(retry.records.two.prepared_rules.resolvedGroupErrors, []);
  assert.deepEqual(retry.records.one.prepared_rules.directErrors, []);
  const stillUnknown = run({ one: meta({ app_groups: ['sw', 'unknown'] }) }, 'retry', {}, { metadataRetryIds: ['one'] });
  assert.match(stillUnknown.records.one.prepared_rules.metadataErrors[0].msg, /二次确认后/);
});

test('known non-white groups remain ignored, while unknown IDs are not ignored', () => {
  const { records } = run({ ok: meta({ app_groups: ['sw', 'outside'] }), bad: meta({ app_groups: ['sw', 'iaa', 'outside'] }), unknown: meta({ app_groups: ['sw', 'unknown'] }) });
  assert.deepEqual(records.ok.prepared_rules.directErrors, []);
  assert.match(records.bad.prepared_rules.directErrors[0].msg, /已忽略非白名单组=外部联运组/);
  assert.ok(records.unknown.prepared_rules.metadataErrors.length);
});

test('classification date cutoff and all switches are preserved in collection', () => {
  for (const created_at of ['2026-05-31 23:59:59', '', 'invalid']) {
    const { records } = run({ one: meta({ created_at, cn_name: '剧(葡配)', copyright_expire_at: null }) });
    assert.deepEqual(records.one.prepared_rules.classificationErrors, []);
    assert.ok(records.one.prepared_rules.directErrors.some(e => e.type === 'copyright'));
  }
  const { records } = run({ one: meta({ cn_name: '剧(葡配)', unlock_type: 'bad', copyright_expire_at: null }) }, 'initial', Object.fromEntries(Object.keys(ruleSettings()).map(key => [key, true])));
  assert.deepEqual(records.one.prepared_rules.directErrors, []);
  assert.deepEqual(records.one.prepared_rules.classificationErrors, []);
});
