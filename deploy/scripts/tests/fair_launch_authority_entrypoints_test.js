'use strict';
const assert=require('assert');
const cp=require('child_process');
const path=require('path');
const root=path.resolve(__dirname,'../..');
function run(script,args=[],env={}){return cp.spawnSync(process.execPath,[path.join(root,'scripts/fair-launch',script),...args],{encoding:'utf8',env:{...process.env,...env}});}
for(const [script,args] of [['render-schema-assertions.js',[]],['render-resume-assertions.js',[]]]){
  const r=run(script,args);assert.strictEqual(r.status,78,script);assert.strictEqual(r.stderr.trim(),'fair_launch_authority_unavailable');
}
const manifest=path.join(root,'config/fair-launch-dependency-manifest.example.json');
const dep=run('verify-dependency-manifest.js',[manifest,'--require-admission']);
assert.notStrictEqual(dep.status,0);assert.match(dep.stderr,/fair_launch_authority_unavailable/);
const collect=cp.spawnSync('bash',[path.join(root,'scripts/fair-launch/collect-source-registry-candidate.sh')],{encoding:'utf8'});
assert.strictEqual(collect.status,78);assert.match(collect.stderr,/fair_launch_candidate_collection_unavailable/);assert.doesNotMatch(collect.stderr,/candidate generated/);
for(const script of ['generate-schema-manifest.js','generate-resume-checkpoint.js']){
  const marker=path.join(root,'migrations/verify',`.must-not-write-${script}`);
  const r=run(script,['unexpected-argument'],{FAIR_LAUNCH_DISPOSABLE_MYSQL_DSN:'must-not-read',JC_FAIR_LAUNCH_MYSQL_SNAPSHOT_CONFIRM:'OBSERVED_DISPOSABLE_MYSQL8',OUTPUT_PATH:marker});
  assert.strictEqual(r.status,78,script);assert.strictEqual(r.stderr.trim(),'fair_launch_candidate_collection_unavailable');assert.strictEqual(r.stdout,'');
  assert.strictEqual(require('fs').existsSync(marker),false,script+' wrote output');
}
console.log('Fair Launch unavailable authority entrypoints: PASS');
