'use strict';
const {runDeploymentChecks}=require('../internal/fairLaunchDeploymentChecks');
const {validateTestApproval}=require('./fairLaunchAuthorityValidation');

async function runTestValidation({approvalPath,authorityPath,authorityOptions={}}){
  const checks=await runDeploymentChecks();
  const approval=validateTestApproval(checks.actual,approvalPath,authorityPath,authorityOptions);
  const result={...checks,approval};
  console.log(JSON.stringify({deployable:false,validationOnly:true,compiler:checks.actual.compiler,sizes:checks.sizes,selectorCount:checks.selectorCount,manifest:checks.actual,localFixtureVerified:true,approval,negativeCodehashTest:true},null,2));
  return result;
}
module.exports={runTestValidation};
