// The public detail API uses totalRaised and does not include an `id` field.
// Preserve the route ID as a string: Sonyflake IDs exceed JS safe integers.
export function toFundingPanelProject(project, routeID) {
  return {
    ...project,
    id: String(routeID),
    raised: project?.totalRaised ?? project?.raised ?? '0',
    // Backend detail times are Unix seconds. Never substitute launchAt or
    // multiply by 1000 for a V3 funding window/contract identity.
    startTime: project?.startTime ?? project?.start_time,
    endTime: project?.endTime ?? project?.end_time,
  }
}
