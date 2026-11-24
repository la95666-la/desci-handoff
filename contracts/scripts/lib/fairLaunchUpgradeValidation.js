'use strict';
const {assertProductionValidatorEnvironment}=require('../fairLaunchArchitecture');
const {validateProductionApproval}=require('./fairLaunchProductionAuthorityValidation');
const {prepareUpgradeValidation,completeUpgradeValidation}=require('./internal/fairLaunchUpgradeChecks');

async function runValidation(){
  if(arguments.length!==0)throw new Error('production upgrade validation does not accept arguments');
  assertProductionValidatorEnvironment();
  const context=await prepareUpgradeValidation();
  const approval=validateProductionApproval(context.actualUpgradeManifest,process.env.JC_FAIR_LAUNCH_UPGRADE_APPROVAL_FILE);
  return completeUpgradeValidation(context,approval);
}
module.exports={runValidation};
