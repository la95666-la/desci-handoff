'use strict';
const { ethers, network, artifacts }=require('hardhat');
const { facets, PRODUCTION_COMPILER, selectorPlan, deployFacetSet, manifest, validateLocalFixture }=require('../../fairLaunchArchitecture');
const localFixture=require('../../validation/fair-launch-local-deployment-manifest.json');
async function runDeploymentChecks(){

 if(network.name!=='hardhat')throw new Error('validation is local-only; network deployment is forbidden');
 const d=await deployFacetSet(); const actual=await manifest(d); const plan=await selectorPlan();
 if(JSON.stringify(actual.compiler)!==JSON.stringify(PRODUCTION_COMPILER))throw new Error('production compiler identity mismatch');
 validateLocalFixture(actual, localFixture);
 const tampered=structuredClone(localFixture); tampered.components[0].runtimeCodeHash=ethers.ZeroHash;
 try{validateLocalFixture(actual,tampered);throw new Error('tampered local fixture accepted')}catch(e){if(e.message==='tampered local fixture accepted')throw e;}
 const contracts=[['dispatcher','JoinCareFairLaunchV1'],...facets]; const sizes=[];
 for(const [key,name] of contracts){const a=await artifacts.readArtifact(name);const size=(a.deployedBytecode.length-2)/2;sizes.push({key,name,runtimeSizeBytes:size,maximum:23000,pass:size<=23000});if(size>23000)throw new Error(`${name} size ${size} > 23000`);}
 return {actual,sizes,selectorCount:plan.length};
}
module.exports={runDeploymentChecks};
