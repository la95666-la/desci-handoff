'use strict';
// Direct MySQL resume-checkpoint collection is not admitted in this release.
// Reject before reading arguments, environment variables, DSNs or output paths.
process.stderr.write('fair_launch_candidate_collection_unavailable\n');
process.exit(78);
