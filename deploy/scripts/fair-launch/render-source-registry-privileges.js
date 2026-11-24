'use strict';
const {grantStatements,quoteAccount,DEFINER,INGEST,AUDITOR,assertDistinctIdentities,RUNTIME_READ_ONLY,RUNTIME_MUTABLE,ADMIN_MUTABLE,JOB_MUTABLE,AUDIT_READ_ONLY}=require('./source-registry-privilege-policy');
const [,,phase,database]=process.argv;
if(!['identities','grants','tables'].includes(phase)||!/^[A-Za-z0-9_]+$/.test(database||''))throw Error('invalid Source Registry privilege render arguments');
const id=(prefix,fallbackHost='%')=>({user:process.env[`${prefix}_USER`],host:process.env[`${prefix}_HOST`]||fallbackHost});
const identities={runtime:id('JC_FL_RUNTIME'),admin:id('JC_FL_ADMIN'),job:id('JC_FL_JOB'),migration:id('JC_FL_MIGRATION'),schemaAdmin:id('JC_FL_SCHEMA_ADMIN'),auditor:AUDITOR,definer:DEFINER,ingest:INGEST};
for(const [name,x] of Object.entries(identities))if(!/^[A-Za-z0-9_]+$/.test(x.user||'')||!/^[A-Za-z0-9_.:%-]+$/.test(x.host||''))throw Error(`${name} identity invalid`);
assertDistinctIdentities(identities);
if(phase==='tables')process.stdout.write([...new Set([...RUNTIME_READ_ONLY,...RUNTIME_MUTABLE,...ADMIN_MUTABLE,...JOB_MUTABLE,...AUDIT_READ_ONLY])].sort().join('\n')+'\n');
else if(phase==='identities'){
 const password=process.env.JC_FAIR_LAUNCH_SOURCE_INGEST_PASSWORD,auditorPassword=process.env.JC_FAIR_LAUNCH_AUDITOR_PASSWORD;
 for(const [label,value] of [['ingestion',password],['auditor',auditorPassword]])if(!value||value.length<24||/placeholder|changeme|example/i.test(value))throw Error(`${label} credential missing/placeholder/too short`);
 const esc=String(password).replaceAll("'","''"),auditEsc=String(auditorPassword).replaceAll("'","''");
 process.stdout.write(`CREATE USER IF NOT EXISTS ${quoteAccount(DEFINER)} ACCOUNT LOCK;\nALTER USER ${quoteAccount(DEFINER)} ACCOUNT LOCK;\nCREATE USER IF NOT EXISTS ${quoteAccount(AUDITOR)} IDENTIFIED BY '${auditEsc}';\nCREATE USER IF NOT EXISTS ${quoteAccount(INGEST)} IDENTIFIED BY '${esc}';\n`);
}else process.stdout.write(grantStatements(database,identities).join(';\n')+';\n');
