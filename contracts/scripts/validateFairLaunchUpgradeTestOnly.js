'use strict';
const {runTestValidation}=require('./lib/test-only/fairLaunchUpgradeValidation');
async function main(){
  const authorityPath=process.env.JC_FAIR_LAUNCH_TEST_AUTHORITY_FILE;
  const approvalPath=process.env.JC_FAIR_LAUNCH_TEST_APPROVAL_FILE||process.env.JC_FAIR_LAUNCH_UPGRADE_APPROVAL_FILE;
  await runTestValidation({approvalPath,authorityPath});
}
main().catch(error=>{console.error(error);process.exitCode=1});
