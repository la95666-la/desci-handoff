'use strict';
const assert=require('assert');
const {verifyAggregateAbi,verifyActualRouting,selectorPlan,sortedWireAbi,digest,reviewerExpectedAbi}=require('./fairLaunchArchitecture');
async function main(){
 const golden=require('./audit/golden/abi.golden.json');
 const aggregate=await verifyAggregateAbi();
 assert.deepStrictEqual(sortedWireAbi(aggregate),sortedWireAbi(reviewerExpectedAbi(golden)),'full aggregate ABI reviewer-approved named contract mismatch');
 const actual=await verifyActualRouting(), expected=await selectorPlan();
 assert.deepStrictEqual(actual.map(({key,signature,selector})=>({key,signature,selector})),expected.map(({key,signature,selector})=>({key,signature,selector})),'actual fallback selector routing mismatch');
 const fs=require('fs'),crypto=require('crypto');const text=fs.readFileSync('contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol','utf8');
 assert(text.includes('0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700'));
 const members=require('./audit/golden/storage-layout.golden.json').members.map(x=>x.label);let cursor=text.indexOf('struct FairLaunchStorage');
 for(const label of members){const next=text.indexOf(label,cursor);assert(next>cursor,`storage member missing/reordered ${label}`);cursor=next}
 const normalized=text.slice(text.indexOf('struct FairLaunchStorage'),text.indexOf('}',text.indexOf('struct FairLaunchStorage'))+1).replace(/\s+/g,' ');
 const storageSourceDigest=crypto.createHash('sha256').update(normalized).digest('hex');
 console.log(`Fair Launch production freeze PASS: ${golden.length} structurally identical ABI entries, ${actual.length} mechanically verified immutable routed selectors, aggregate ${digest(sortedWireAbi(aggregate))}, namespace preserved, source digest ${storageSourceDigest}`)
}
main().catch(e=>{console.error(e);process.exitCode=1});
