'use strict';
const {runValidation}=require('./lib/fairLaunchDeploymentValidation');
async function main(){await runValidation();}
main().catch(error=>{console.error(error);process.exitCode=1});
