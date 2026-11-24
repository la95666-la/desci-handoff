'use strict';
const assert=require('assert');
const cp=require('child_process');
const path=require('path');
const decision=require('../fair-launch/activation-decision');

assert.strictEqual(decision.ERROR,'fair_launch_activation_unavailable');
assert.throws(()=>decision.decide(),/fair_launch_activation_unavailable/);
for(const name of ['JC_FAIR_LAUNCH_TEST_AUTHORITY','jc_fair_launch_test_authority','Jc_Fair_Launch_Test_Probe']){
  assert.throws(()=>decision.rejectTestAuthorityEnvironment({[name]:'1'}),/forbidden/);
}
const result=cp.spawnSync(process.execPath,[path.join(__dirname,'../fair-launch/activation-decision.js')],{encoding:'utf8',env:{PATH:process.env.PATH||'',SystemRoot:process.env.SystemRoot||'',WINDIR:process.env.WINDIR||''}});
assert.strictEqual(result.status,78);
assert.strictEqual(result.stderr.trim(),'fair_launch_activation_unavailable');
assert.strictEqual(result.stdout,'');
console.log('Fair Launch activation decision unavailable: PASS');
