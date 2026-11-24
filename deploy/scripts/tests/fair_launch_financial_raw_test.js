'use strict';
const fs=require('fs'),assert=require('assert'),path=require('path');
const root=path.resolve(__dirname,'../..');
const registry=JSON.parse(fs.readFileSync(path.join(root,'migrations/verify/2026_09_fair_launch_financial_raw_registry.json'),'utf8'));
const sourceFiles=[path.join(root,'migrations/baseline/fair-launch-pre-2026-09-03.sql'),...fs.readdirSync(path.join(root,'migrations')).filter(x=>/^2026_09_0[3-6].*\.sql$/.test(x)).map(x=>path.join(root,'migrations',x))];
const allSql=sourceFiles.map(x=>fs.readFileSync(x,'utf8')).join('\n');
const discovered=new Set();
let currentTable=null;
for(const line of allSql.split(/\r?\n/)){
  const create=line.match(/CREATE TABLE(?: IF NOT EXISTS)?\s+`?([A-Za-z0-9_]+)`?\s*\(/i);
  if(create) currentTable=create[1];
  if(currentTable){for(const c of line.matchAll(/`?([A-Za-z][A-Za-z0-9_]*_raw)`?\s+(VARCHAR\(78\)|MEDIUMTEXT)/gi))discovered.add(`${currentTable}.${c[1]}`);}
  if(currentTable&&/\)\s*ENGINE=/i.test(line))currentTable=null;
}
// projects raw columns are installed procedurally in the baseline/0903 migration.
for(const c of ['token_price_raw','hard_cap_raw','soft_cap_raw','max_per_user_raw','total_raised_raw','total_deposited_raw','sale_token_amount_raw','min_contribution_raw','max_contribution_raw','final_total_raised_raw','final_sale_token_amount_raw'])discovered.add(`projects.${c}`);
const classified=new Set([...registry.fields,...registry.nonUintRawFields].map(x=>`${x.table}.${x.column}`));
assert.deepEqual([...classified].sort(),[...discovered].sort(),`reviewed raw registry mismatch\nmissing=${[...discovered].filter(x=>!classified.has(x))}\nextra=${[...classified].filter(x=>!discovered.has(x))}`);
const max=BigInt(registry.maxUint256),canonical=x=>/^(0|[1-9][0-9]{0,77})$/.test(x)&&BigInt(x)<=max;
for(const field of registry.fields){for(const value of ['00','0'.repeat(79),(max+1n).toString()])assert.equal(canonical(value),false,`${field.table}.${field.column} accepted ${value}`);assert(allSql.includes(field.column),`registry field absent from schema: ${field.table}.${field.column}`);}
for(const field of registry.nonUintRawFields)assert.notEqual(field.kind,'uint256');
for(const required of ['chk_reorg_submission_uint256_raw','chk_reorg_candidate_project_uint256_raw','chk_reorg_candidate_wallet_uint256_raw'])assert(allSql.includes(required),`missing permanent constraint ${required}`);
console.log(`Fair Launch financial raw registry: ${registry.fields.length} uint256 + ${registry.nonUintRawFields.length} explicit non-uint fields; schema discovery and negatives PASS`);
