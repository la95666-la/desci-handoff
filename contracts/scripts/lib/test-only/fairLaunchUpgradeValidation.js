'use strict';
const {prepareUpgradeValidation,completeUpgradeValidation}=require('../internal/fairLaunchUpgradeChecks');
const {validateTestApproval}=require('./fairLaunchAuthorityValidation');

async function runTestValidation({approvalPath,authorityPath,authorityOptions={}}){
  const context=await prepareUpgradeValidation();
  const approval=validateTestApproval(context.actualUpgradeManifest,approvalPath,authorityPath,authorityOptions);
  return completeUpgradeValidation(context,approval);
}
module.exports={runTestValidation};
