import type { ProjectListRow } from '#/api/joincare/project_content';

import { isLegacyV2FundLedgerSelectable } from '../projects/funding-mode';

export interface LegacyV2ProjectPickerState {
  projectId: string | undefined;
  invalidateLoadedProject: () => void;
}

export function legacyV2ProjectOptions(
  projects: ProjectListRow[],
): ProjectListRow[] {
  return projects.filter((project) => isLegacyV2FundLedgerSelectable(project));
}

export function clearLegacyV2ProjectSelection(
  state: LegacyV2ProjectPickerState,
): void {
  state.projectId = undefined;
  state.invalidateLoadedProject();
}

export function reconcileLegacyV2ProjectSelection(
  state: LegacyV2ProjectPickerState,
  options: ProjectListRow[],
): void {
  if (
    state.projectId !== undefined &&
    !options.some((project) => project.project_id === state.projectId)
  ) {
    clearLegacyV2ProjectSelection(state);
  }
}
