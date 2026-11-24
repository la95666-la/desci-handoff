#!/usr/bin/env bash
# Remote configuration upload is unavailable while Backend/Job activation and
# secret transport remain unadmitted. This prevents unsafe path interpolation.
set -euo pipefail
source "$JC_LIB/colors.sh"
error "remote_config_upload_unavailable"
exit 78
