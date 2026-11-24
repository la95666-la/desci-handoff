'use strict';
const crypto = require('crypto');
const {MIGRATIONS,RESUMABLE_MIGRATIONS,assertSnapshotInputs}=require('./migration-inputs');

const SCHEMA_DOMAIN = 'joincare.database.mysql8-information-schema-snapshot.v1';
const CHECKPOINT_DOMAIN = 'joincare.database.mysql8-resume-checkpoint.v1';
function canonical(value) {
  if (value === null) return 'null';
  if (typeof value === 'string' || typeof value === 'boolean') return JSON.stringify(value);
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) throw Error('snapshot contains unsafe number');
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.keys(value).sort().map(k => `${JSON.stringify(k)}:${canonical(value[k])}`).join(',')}}`;
  throw Error('snapshot contains unsupported value');
}
const digest = value => crypto.createHash('sha256').update(typeof value === 'string' ? value : canonical(value)).digest('hex');
const hex64 = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const nonEmpty = value => typeof value === 'string' && value.length > 0;

function uniqueRows(rows, key, label) {
  if (!Array.isArray(rows)) throw Error(`${label} must be an array`);
  const seen = new Set();
  for (const row of rows) {
    const identity = key(row);
    if (!nonEmpty(identity) || seen.has(identity)) throw Error(`${label} duplicate/malformed identity: ${identity || '<empty>'}`);
    seen.add(identity);
  }
}

function validateCommon(snapshot, expectedDomain) {
  if (!snapshot || snapshot.domain !== expectedDomain) throw Error('MySQL snapshot domain mismatch');
  if (snapshot.admissionStatus !== 'MYSQL8_OBSERVED_CANDIDATE') throw Error('snapshot is not a real-MySQL observed candidate');
  if (!/^8\./.test(snapshot.mysqlVersion || '')) throw Error('snapshot was not observed on MySQL 8');
  if (!hex64(snapshot.baselineSha256)) throw Error('snapshot baseline digest invalid');
  if (!snapshot.migrationSha256 || Object.keys(snapshot.migrationSha256).length !== MIGRATIONS.length) throw Error('snapshot migration digest set incomplete');
  for (const name of MIGRATIONS) if (!hex64(snapshot.migrationSha256[name])) throw Error(`snapshot migration digest invalid: ${name}`);
  if (!hex64(snapshot.observationDigest)) throw Error('snapshot observation digest invalid');
  if (!hex64(snapshot.checksumManifestDigest)) throw Error('snapshot checksum-manifest digest invalid');
  assertSnapshotInputs(snapshot);
}

function validateDefinitions(snapshot) {
  const d = snapshot.definitions;
  if (!d || !d.expectedCounts) throw Error('snapshot definitions/counts missing');
  const groups = ['tables','columns','indexes','foreignKeys','checks','views','triggers','routines','grants'];
  for (const group of groups) {
    if (!Number.isSafeInteger(d.expectedCounts[group]) || d.expectedCounts[group] < 0) throw Error(`snapshot expected ${group} count invalid`);
    if (!Array.isArray(d[group]) || d[group].length !== d.expectedCounts[group]) throw Error(`snapshot exact ${group} count mismatch`);
  }
  uniqueRows(d.tables, x => x.tableName, 'tables');
  uniqueRows(d.columns, x => `${x.tableName}\0${x.columnName}`, 'columns');
  uniqueRows(d.indexes, x => `${x.tableName}\0${x.indexName}\0${x.sequence}`, 'indexes');
  uniqueRows(d.foreignKeys, x => `${x.tableName}\0${x.constraintName}\0${x.ordinal}`, 'foreign keys');
  uniqueRows(d.checks, x => `${x.tableName}\0${x.constraintName}`, 'checks');
  uniqueRows(d.views, x => x.viewName, 'views');
  uniqueRows(d.triggers, x => x.triggerName, 'triggers');
  uniqueRows(d.routines, x => x.routineName, 'routines');
  uniqueRows(d.grants, x => `${x.grantee}\0${x.privilegeType}\0${x.objectName}`, 'grants');
  for (const t of d.tables) if (!['BASE TABLE'].includes(t.tableType) || !nonEmpty(t.engine) || !nonEmpty(t.tableCollation)) throw Error('table definition malformed');
  const tableNames=new Set(d.tables.map(x=>x.tableName)),viewNames=new Set(d.views.map(x=>x.viewName));
  for(const name of viewNames)if(tableNames.has(name))throw Error('view is also collected as a base table');
  for (const c of d.columns) if (!tableNames.has(c.tableName) || !nonEmpty(c.columnName) || !nonEmpty(c.columnType) || !['YES','NO'].includes(c.isNullable) || !Number.isSafeInteger(c.ordinal) || c.ordinal < 1 || typeof c.extra !== 'string' || typeof c.generationExpression !== 'string') throw Error('column definition malformed');
  for (const i of d.indexes) if (!tableNames.has(i.tableName) || !nonEmpty(i.indexName) || !Number.isSafeInteger(i.sequence) || i.sequence < 1 || ![0,1].includes(i.nonUnique) || !nonEmpty(i.columnExpression) || !nonEmpty(i.indexType)) throw Error('index definition malformed');
  for (const f of d.foreignKeys) if (!nonEmpty(f.tableName) || !nonEmpty(f.constraintName) || !Number.isSafeInteger(f.ordinal) || f.ordinal < 1 || !nonEmpty(f.columnName) || !nonEmpty(f.referencedTableName) || !nonEmpty(f.referencedColumnName) || !nonEmpty(f.updateRule) || !nonEmpty(f.deleteRule)) throw Error('foreign-key definition malformed');
  for (const c of d.checks) if (!nonEmpty(c.tableName) || !nonEmpty(c.constraintName) || !nonEmpty(c.checkClause) || c.enforced !== 'YES') throw Error('check definition malformed');
  for (const v of d.views) if (!nonEmpty(v.viewName) || !nonEmpty(v.viewDefinition) || !nonEmpty(v.checkOption) || !nonEmpty(v.securityType)) throw Error('view definition malformed');
  for (const t of d.triggers) if (!nonEmpty(t.triggerName) || !nonEmpty(t.eventManipulation) || !nonEmpty(t.actionTiming) || !nonEmpty(t.eventObjectTable) || !Number.isSafeInteger(t.actionOrder) || t.actionOrder < 1 || !nonEmpty(t.definer) || /placeholder|example|changeme/i.test(t.definer) || !nonEmpty(t.actionStatement)) throw Error('trigger definition malformed');
  for (const r of d.routines) if (!nonEmpty(r.routineName) || r.routineType!=='PROCEDURE' || !nonEmpty(r.sqlDataAccess) || !nonEmpty(r.securityType) || !nonEmpty(r.definer) || /placeholder|example|changeme/i.test(r.definer) || !nonEmpty(r.routineDefinition)) throw Error('routine definition malformed');
  for (const g of d.grants) if (!nonEmpty(g.grantee) || !nonEmpty(g.privilegeType) || !['YES','NO'].includes(g.isGrantable) || !nonEmpty(g.objectName) || /placeholder|example|changeme/i.test(g.grantee)) throw Error('grant definition malformed');
}

function validateSchemaSnapshot(snapshot) {
  validateCommon(snapshot, SCHEMA_DOMAIN);
  if (snapshot.phase !== 'post_2026_09_07') throw Error('final schema phase mismatch');
  validateDefinitions(snapshot);
  if (snapshot.observationDigest !== digest(snapshot.definitions)) throw Error('schema observation digest mismatch');
  const unsigned = structuredClone(snapshot); delete unsigned.snapshotDigest;
  const calculated = digest(unsigned);
  if (!hex64(snapshot.snapshotDigest) || snapshot.snapshotDigest !== calculated) throw Error('schema snapshot digest mismatch');
  return calculated;
}

function validateCheckpoint(snapshot, phase) {
  validateCommon(snapshot, CHECKPOINT_DOMAIN);
  if (!RESUMABLE_MIGRATIONS.includes(phase) || snapshot.phase !== phase) throw Error('resume checkpoint phase mismatch');
  if (!snapshot.completedMigrations || !snapshot.futureMigrations || !snapshot.partialObjectContract || !snapshot.partialContentPolicy) throw Error('resume checkpoint contract incomplete');
  const expectedPartial={
    '2026_09_03_fair_launch_project_model.sql':{kind:'column',tableName:'projects',objectName:'funding_mode'},
    '2026_09_04_fair_launch_ce_v2_governance.sql':{kind:'table',tableName:'chain_contract_source',objectName:'chain_contract_source'},
    '2026_09_05_fair_launch_signing_release_control.sql':{kind:'table',tableName:'fair_launch_signing_generation',objectName:'fair_launch_signing_generation'},
    '2026_09_06_fair_launch_action_execution.sql':{kind:'table',tableName:'fair_launch_action_request',objectName:'fair_launch_action_request'},
  }[phase];
  if (canonical(snapshot.partialObjectContract)!==canonical(expectedPartial)) throw Error('resume partial-object contract mismatch');
  const index = MIGRATIONS.indexOf(phase);
  if (canonical(snapshot.completedMigrations) !== canonical(RESUMABLE_MIGRATIONS.slice(0,index))) throw Error('resume completed-prefix mismatch');
  if (canonical(snapshot.futureMigrations) !== canonical(MIGRATIONS.slice(index+1))) throw Error('resume future-suffix mismatch');
  if (!hex64(snapshot.phaseDigest)) throw Error('resume phase digest invalid');
  validateDefinitions(snapshot);
  if (snapshot.observationDigest !== digest(snapshot.definitions)) throw Error('resume observation digest mismatch');
  const unsigned = structuredClone(snapshot); delete unsigned.snapshotDigest;
  const calculated = digest(unsigned);
  if (!hex64(snapshot.snapshotDigest) || snapshot.snapshotDigest !== calculated) throw Error('resume snapshot digest mismatch');
  validatePartialContentPolicy(snapshot.partialContentPolicy);
  const expectedContent={
    '2026_09_03_fair_launch_project_model.sql':[{query:'SELECT COUNT(*) FROM projects WHERE funding_mode IS NOT NULL',expected:0}],
    '2026_09_04_fair_launch_ce_v2_governance.sql':[{query:'SELECT COUNT(*) FROM chain_contract_source',expected:0}],
    '2026_09_05_fair_launch_signing_release_control.sql':[{query:'SELECT COUNT(*) FROM fair_launch_signing_generation',expected:0}],
    '2026_09_06_fair_launch_action_execution.sql':[{query:'SELECT COUNT(*) FROM fair_launch_action_request',expected:0}],
  }[phase];
  if(canonical(snapshot.partialContentPolicy)!==canonical(expectedContent))throw Error('resume partial content policy mismatch');
  const phaseUnsigned = {phase:snapshot.phase,completedMigrations:snapshot.completedMigrations,futureMigrations:snapshot.futureMigrations,partialObjectContract:snapshot.partialObjectContract,partialContentPolicy:snapshot.partialContentPolicy,definitions:snapshot.definitions};
  if (snapshot.phaseDigest !== digest(phaseUnsigned)) throw Error('resume phase digest mismatch');
  return calculated;
}

const q = value => `'${String(value).replaceAll("'", "''")}'`;
function add(out, query, expected=1) { out.push(`SET @JC_FL_SHAPE_MISMATCHES=@JC_FL_SHAPE_MISMATCHES+IF((${query})=${expected},0,1);`); }
function validatePartialContentPolicy(policy){
  if(!Array.isArray(policy) || policy.length<1)throw Error('resume partial content policy missing');
  for(const rule of policy){if(!rule||typeof rule.query!=='string'||!/^SELECT COUNT\(\*\) FROM [a-zA-Z0-9_` .='()<>-]+$/.test(rule.query)||!Number.isSafeInteger(rule.expected)||rule.expected<0)throw Error('resume partial content policy malformed');}
}
function renderPartialContentAssertions(snapshot, variable='@JC_FL_RESUME_NONEMPTY_PARTIALS'){
  validatePartialContentPolicy(snapshot.partialContentPolicy);
  const lines=[`SET ${variable}=0;`];
  for(const rule of snapshot.partialContentPolicy)lines.push(`SET ${variable}=${variable}+IF(((${rule.query}))=${rule.expected},0,1);`);
  return lines.join('\n')+'\n';
}
function renderDefinitionAssertions(snapshot, variable='@JC_FL_SHAPE_MISMATCHES') {
  const lines=[`SET ${variable}=0;`], d=snapshot.definitions;
  const addLine=(query,expected=1)=>lines.push(`SET ${variable}=${variable}+IF((${query})=${expected},0,1);`);
  addLine(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`,d.expectedCounts.tables);
  addLine(`SELECT COUNT(*) FROM information_schema.COLUMNS c JOIN information_schema.TABLES t ON t.TABLE_SCHEMA=c.TABLE_SCHEMA AND t.TABLE_NAME=c.TABLE_NAME WHERE c.TABLE_SCHEMA=DATABASE() AND t.TABLE_TYPE='BASE TABLE'`,d.expectedCounts.columns);
  addLine(`SELECT COUNT(*) FROM information_schema.STATISTICS s JOIN information_schema.TABLES t ON t.TABLE_SCHEMA=s.TABLE_SCHEMA AND t.TABLE_NAME=s.TABLE_NAME WHERE s.TABLE_SCHEMA=DATABASE() AND t.TABLE_TYPE='BASE TABLE'`,d.expectedCounts.indexes);
  addLine(`SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND REFERENCED_TABLE_NAME IS NOT NULL`,d.expectedCounts.foreignKeys);
  addLine(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='CHECK'`,d.expectedCounts.checks);
  addLine(`SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE()`,d.expectedCounts.views);
  addLine(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()`,d.expectedCounts.triggers);
  addLine(`SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE()`,d.expectedCounts.routines);
  if(d.expectedCounts.grants!==0)throw Error('effective grants must be admitted by the separate signed privilege manifest');
  for(const t of d.tables)addLine(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=${q(t.tableName)} AND TABLE_TYPE=${q(t.tableType)} AND ENGINE=${q(t.engine)} AND TABLE_COLLATION=${q(t.tableCollation)}`);
  for(const c of d.columns)addLine(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=${q(c.tableName)} AND COLUMN_NAME=${q(c.columnName)} AND ORDINAL_POSITION=${c.ordinal} AND LOWER(COLUMN_TYPE)=${q(c.columnType.toLowerCase())} AND IS_NULLABLE=${q(c.isNullable)} AND COALESCE(COLUMN_DEFAULT,'<NULL>')=${q(c.columnDefault===null?'<NULL>':c.columnDefault)} AND COALESCE(CHARACTER_SET_NAME,'<NULL>')=${q(c.characterSetName===null?'<NULL>':c.characterSetName)} AND COALESCE(COLLATION_NAME,'<NULL>')=${q(c.collationName===null?'<NULL>':c.collationName)} AND EXTRA=${q(c.extra)} AND COALESCE(GENERATION_EXPRESSION,'')=${q(c.generationExpression)}`);
  for(const i of d.indexes)addLine(`SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=${q(i.tableName)} AND INDEX_NAME=${q(i.indexName)} AND SEQ_IN_INDEX=${i.sequence} AND NON_UNIQUE=${i.nonUnique} AND COALESCE(COLUMN_NAME,EXPRESSION)=${q(i.columnExpression)} AND INDEX_TYPE=${q(i.indexType)}`);
  for(const f of d.foreignKeys)addLine(`SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME WHERE k.CONSTRAINT_SCHEMA=DATABASE() AND k.TABLE_NAME=${q(f.tableName)} AND k.CONSTRAINT_NAME=${q(f.constraintName)} AND k.ORDINAL_POSITION=${f.ordinal} AND k.COLUMN_NAME=${q(f.columnName)} AND k.REFERENCED_TABLE_NAME=${q(f.referencedTableName)} AND k.REFERENCED_COLUMN_NAME=${q(f.referencedColumnName)} AND r.UPDATE_RULE=${q(f.updateRule)} AND r.DELETE_RULE=${q(f.deleteRule)}`);
  for(const c of d.checks)addLine(`SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS cc JOIN information_schema.TABLE_CONSTRAINTS tc ON tc.CONSTRAINT_SCHEMA=cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME=cc.CONSTRAINT_NAME WHERE cc.CONSTRAINT_SCHEMA=DATABASE() AND tc.TABLE_NAME=${q(c.tableName)} AND cc.CONSTRAINT_NAME=${q(c.constraintName)} AND cc.CHECK_CLAUSE=${q(c.checkClause)} AND tc.ENFORCED=${q(c.enforced)}`);
  for(const v of d.views)addLine(`SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=${q(v.viewName)} AND VIEW_DEFINITION=${q(v.viewDefinition)} AND CHECK_OPTION=${q(v.checkOption)} AND SECURITY_TYPE=${q(v.securityType)}`);
  for(const t of d.triggers)addLine(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME=${q(t.triggerName)} AND EVENT_MANIPULATION=${q(t.eventManipulation)} AND ACTION_TIMING=${q(t.actionTiming)} AND EVENT_OBJECT_TABLE=${q(t.eventObjectTable)} AND ACTION_ORDER=${t.actionOrder} AND DEFINER=${q(t.definer)} AND ACTION_STATEMENT=${q(t.actionStatement)}`);
  for(const r of d.routines)addLine(`SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME=${q(r.routineName)} AND ROUTINE_TYPE=${q(r.routineType)} AND SQL_DATA_ACCESS=${q(r.sqlDataAccess)} AND SECURITY_TYPE=${q(r.securityType)} AND DEFINER=${q(r.definer)} AND ROUTINE_DEFINITION=${q(r.routineDefinition)}`);
  return lines.join('\n')+'\n';
}

module.exports={SCHEMA_DOMAIN,CHECKPOINT_DOMAIN,MIGRATIONS,RESUMABLE_MIGRATIONS,canonical,digest,validateSchemaSnapshot,validateCheckpoint,renderDefinitionAssertions,renderPartialContentAssertions};
