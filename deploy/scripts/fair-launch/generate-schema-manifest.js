'use strict';
// Direct MySQL candidate collection is not admitted in this release. Reject
// before reading environment variables, DSNs, MySQL state or output paths.
process.stderr.write('fair_launch_candidate_collection_unavailable\n');
process.exit(78);
