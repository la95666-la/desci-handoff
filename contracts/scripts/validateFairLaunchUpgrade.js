'use strict';
const {runValidation}=require('./lib/fairLaunchUpgradeValidation');
async function main(){await runValidation();}
main().catch(error=>{console.error(error);process.exitCode=1});
