'use strict';
const {assertProductionValidatorEnvironment,validateApprovalArtifact}=require('../fairLaunchArchitecture');

function validateProductionApproval(actual,approvalPath){
  if(arguments.length!==2)throw new Error('production authority validation requires exactly manifest and approval path');
  assertProductionValidatorEnvironment();
  const approval=validateApprovalArtifact(actual,approvalPath);
  if(!approval.productionAuthorized)throw new Error('production validation requires production reviewer authority');
  return approval;
}
module.exports={validateProductionApproval};
