'use strict';
// Retired production facade. Live privilege/account/definer evidence requires a
// reviewer-owned external database auditor; application identities are intentionally
// unable to inspect mysql.user, default roles, or grants for other accounts.
const DOMAIN='joincare.database.fair-launch-source-registry-post-state.v1';
function collect(){ throw new Error('fair_launch_candidate_collection_unavailable'); }
function environmentIdentities(){
  const {DEFINER,INGEST,AUDITOR,assertDistinctIdentities}=require('./source-registry-privilege-policy');
  const id=(prefix,host='%')=>({user:process.env[`${prefix}_USER`],host:process.env[`${prefix}_HOST`]||host});
  const identities={runtime:id('JC_FL_RUNTIME'),admin:id('JC_FL_ADMIN'),job:id('JC_FL_JOB'),migration:id('JC_FL_MIGRATION'),schemaAdmin:id('JC_FL_SCHEMA_ADMIN'),auditor:AUDITOR,definer:DEFINER,ingest:INGEST};
  for(const [name,value] of Object.entries(identities))if(!/^[A-Za-z0-9_]+$/.test(value.user||'')||!/^[A-Za-z0-9_.:%-]+$/.test(value.host||''))throw Error(`${name} identity invalid`);
  assertDistinctIdentities(identities);return identities;
}
module.exports={DOMAIN,collect,environmentIdentities};
