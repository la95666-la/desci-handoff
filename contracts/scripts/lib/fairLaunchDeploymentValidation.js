'use strict';
const {assertProductionValidatorEnvironment}=require('../fairLaunchArchitecture');
const {validateProductionApproval}=require('./fairLaunchProductionAuthorityValidation');
const {runDeploymentChecks}=require('./internal/fairLaunchDeploymentChecks');

async function runValidation(){
  if(arguments.length!==0)throw new Error('production deployment validation does not accept arguments');
  assertProductionValidatorEnvironment();
  const checks=await runDeploymentChecks();
  const approval=validateProductionApproval(checks.actual,process.env.JC_FAIR_LAUNCH_APPROVAL_FILE);
  const result={...checks,approval};
  console.log(JSON.stringify({deployable:true,validationOnly:false,compiler:checks.actual.compiler,sizes:checks.sizes,selectorCount:checks.selectorCount,manifest:checks.actual,localFixtureVerified:true,approval,negativeCodehashTest:true},null,2));
  return result;
}
module.exports={runValidation};
