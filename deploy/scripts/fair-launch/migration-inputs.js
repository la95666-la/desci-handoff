'use strict';
const fs=require('fs'),path=require('path'),crypto=require('crypto');
const ROOT=path.resolve(__dirname,'../..');
const MIGRATIONS=[
  '2026_09_03_fair_launch_project_model.sql',
  '2026_09_04_fair_launch_ce_v2_governance.sql',
  '2026_09_05_fair_launch_signing_release_control.sql',
  '2026_09_06_fair_launch_action_execution.sql',
  '2026_09_07_fair_launch_position_projection.sql',
];
const RESUMABLE_MIGRATIONS=Object.freeze(MIGRATIONS.slice(0,4));
const sha256File=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
function currentMigrationInputs(){
 const baselineFile=path.join(ROOT,'migrations/baseline/fair-launch-pre-2026-09-03.sql');
 const baselineSha256=sha256File(baselineFile);
 const migrationSha256=Object.fromEntries(MIGRATIONS.map(name=>[name,sha256File(path.join(ROOT,'migrations',name))]));
 const allChecksums=fs.readFileSync(path.join(ROOT,'migrations/FAIR_LAUNCH_SHA256SUMS'),'utf8').trim().split(/\r?\n/).filter(Boolean).map(line=>{const m=line.match(/^([0-9a-f]{64}) \*([^\s]+)$/);if(!m)throw Error('migration checksum manifest malformed');return{name:m[2],sha256:m[1]};});
 const checksumManifest=MIGRATIONS.map(name=>allChecksums.find(x=>x.name===name));
 if(checksumManifest.some(row=>!row))throw Error('production migration checksum manifest is incomplete');
 for(const name of MIGRATIONS){const row=checksumManifest.find(x=>x.name===name);if(row.sha256!==migrationSha256[name])throw Error(`migration checksum manifest mismatch: ${name}`);}
 const checksumManifestDigest=crypto.createHash('sha256').update(JSON.stringify(checksumManifest)).digest('hex');
 return{baselineSha256,migrationSha256,checksumManifestDigest};
}
function assertSnapshotInputs(snapshot){
 const current=currentMigrationInputs();
 if(snapshot.baselineSha256!==current.baselineSha256)throw Error('snapshot pinned baseline digest mismatch');
 if(JSON.stringify(snapshot.migrationSha256)!==JSON.stringify(current.migrationSha256))throw Error('snapshot migration digest set differs from current files');
 if(snapshot.checksumManifestDigest!==current.checksumManifestDigest)throw Error('snapshot checksum-manifest digest mismatch');
 return current;
}
module.exports={ROOT,MIGRATIONS,RESUMABLE_MIGRATIONS,currentMigrationInputs,assertSnapshotInputs};
