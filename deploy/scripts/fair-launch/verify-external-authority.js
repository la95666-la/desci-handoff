'use strict';
// Retired compatibility facade. Schema/resume/dependency authority consumers must
// not use a partially bound verifier. This candidate-only release has no admitted
// production authority entrypoint; the chained reviewer authority remains a gate.
const signed=require('./signed-authority');
function unavailable(){throw Error('fair_launch_authority_unavailable');}
module.exports={
  outsideRealFile:unavailable,
  readCanonical:unavailable,
  verifyAuthority:unavailable,
  verifyMonotonicState:unavailable,
  rejectTestAuthorityEnvironment:signed.rejectTestAuthorityEnvironment,
  POLICY_STATUS:signed.PRODUCTION_TRUST_ROOT.status,
};
