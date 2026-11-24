'use strict';
const {validateApprovalArtifact}=require('../../fairLaunchArchitecture');

function validateTestApproval(actual,approvalPath,authorityPath,options={}){
  if(!approvalPath||!authorityPath)throw new Error('test-only validation requires explicit approval and authority files');
  const approval=validateApprovalArtifact(actual,approvalPath,{...options,testMode:true,authorityPath});
  if(approval.productionAuthorized||!approval.testMode)throw new Error('test-only validator accepted production authority');
  return approval;
}
module.exports={validateTestApproval};
