'use strict'
const fs=require('fs'),crypto=require('crypto')
const fail=m=>{throw new Error(m)}
const exact=(x,ks,l)=>{if(!x||typeof x!=='object'||Array.isArray(x)||JSON.stringify(Object.keys(x).sort())!==JSON.stringify([...ks].sort()))fail(l+' keys mismatch')}
const sha=(x,l)=>{if(!/^[0-9a-f]{64}$/.test(String(x||'')))fail(l+' invalid');return x}
const commit=(x,l)=>{if(!/^[0-9a-f]{40}$/.test(String(x||'')))fail(l+' invalid');return x}
const fileSha=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex')
try{
 const [file,manifestFile]=process.argv.slice(2); if(!file||!manifestFile)fail('usage: admission manifest')
 const a=JSON.parse(fs.readFileSync(file,'utf8')),m=JSON.parse(fs.readFileSync(manifestFile,'utf8'))
 if(a.schema!=='joincare.fair-launch.test-server-backend-rollout-admission.v1'||a.status!=='ACTIVE_BACKEND_ROLLOUT_ONLY'||a.environment!=='test-server')fail('identity mismatch')
 exact(a.permissions,['packageBuildPermitted','operationalApplyPermitted','artifactUploadPermitted','databaseMigrationPermitted','serviceMutationPermitted','scannerStartPermitted','materializerStartPermitted','walletSubmissionPermitted','adminDispatchPermitted'],'permissions')
 const expected={packageBuildPermitted:false,operationalApplyPermitted:true,artifactUploadPermitted:true,databaseMigrationPermitted:false,serviceMutationPermitted:true,scannerStartPermitted:false,materializerStartPermitted:false,walletSubmissionPermitted:false,adminDispatchPermitted:false}
 if(JSON.stringify(a.permissions)!==JSON.stringify(expected))fail('permission boundary mismatch')
 exact(a.artifact,['candidateManifestSha256','imageArchiveSha256','imageId','imageTag','backendCommit','deployCommit'],'artifact')
 exact(a.rollback,['currentImageId','requiredTag','healthEndpoint','healthTimeoutSeconds'],'rollback')
 exact(a.rolloutScope,['service','composeProject','composeFile','container','databaseWritesPermitted','jobMutationPermitted','authorityWritesPermitted'],'rolloutScope')
 if(a.rolloutScope.service!=='backend'||a.rolloutScope.container!=='jc-backend'||a.rolloutScope.databaseWritesPermitted||a.rolloutScope.jobMutationPermitted||a.rolloutScope.authorityWritesPermitted)fail('rollout scope mismatch')
 if(fileSha(manifestFile)!==sha(a.artifact.candidateManifestSha256,'manifest sha'))fail('candidate manifest checksum mismatch')
 if(m.status!=='BUILT_NOT_OPERATIONALLY_ADMITTED'||m.permissions.backendReplace||m.permissions.jobStart||m.permissions.scannerStart||m.permissions.materializerStart)fail('candidate is not inert')
 if(commit(m.commits.backend,'manifest backend')!==commit(a.artifact.backendCommit,'backend')||commit(m.commits.deploy,'manifest deploy')!==commit(a.artifact.deployCommit,'deploy'))fail('commit binding mismatch')
 if(m.image.sha256!==sha(a.artifact.imageArchiveSha256,'archive sha')||m.image.imageId!==a.artifact.imageId||m.image.tag!==a.artifact.imageTag)fail('image binding mismatch')
 if(a.rollback.currentImageId!=='sha256:ec2c285c981e1c2bbadf9f3d98b8d2f3cedbe992e82f4f873f602d8c7e929c96'||a.rollback.requiredTag!=='joincare/backend:rollback-pre-acf2d46')fail('rollback binding mismatch')
 console.log(JSON.stringify({status:a.status,admissionSha256:fileSha(file),backendCommit:a.artifact.backendCommit,imageId:a.artifact.imageId,scannerStartPermitted:false}))
}catch(e){console.error('fair_launch_backend_rollout_admission_invalid: '+e.message);process.exit(1)}
