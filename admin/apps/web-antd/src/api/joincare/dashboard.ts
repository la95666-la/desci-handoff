import { requestClient } from '#/api/request';

export interface DashboardOverview {
  user: { new_7d: number; new_today: number; total: number };
  user_growth_30d: { date: string; new_users: number }[];
  visits_30d: {
    date: string;
    unique_visitors: number;
    visits: number;
  }[];
  project: {
    active: number;
    ended: number;
    pre_launch: number;
    total: number;
    warmup: number;
  };
  launchpad: {
    subscribers: number;
    watches: number;
    whitelist: number;
  };
  top_projects: {
    name: string;
    project_id: number;
    subscribers: number;
    total: number;
    watches: number;
    whitelist: number;
  }[];
  checkin: { today: number };
  tg: { active_users_today: number; messages_today: number };
}

type RawTopProject = Partial<DashboardOverview['top_projects'][number]> & {
  Name?: string;
  ProjectID?: number;
  Subscribers?: number;
  Total?: number;
  Watches?: number;
  Whitelist?: number;
};

type RawVisit = Omit<
  DashboardOverview['visits_30d'][number],
  'unique_visitors'
> & {
  unique_visitors?: number;
  uniqueVisitors?: number;
};

type RawDashboardOverview = Omit<
  DashboardOverview,
  'top_projects' | 'visits_30d'
> & {
  top_projects?: RawTopProject[];
  visits_30d?: RawVisit[];
};

export async function getDashboardOverview(): Promise<DashboardOverview> {
  const data = await requestClient.get<RawDashboardOverview>(
    '/v1/dashboard/overview',
    { timeout: 30_000 },
  );

  // 兼容后端滚动发布期间旧版本返回的 Go 字段名（ProjectID / Name 等）。
  const topProjects = (data.top_projects || []).map((project) => ({
    project_id: project.project_id ?? project.ProjectID ?? 0,
    name: project.name ?? project.Name ?? '',
    subscribers: project.subscribers ?? project.Subscribers ?? 0,
    watches: project.watches ?? project.Watches ?? 0,
    whitelist: project.whitelist ?? project.Whitelist ?? 0,
    total: project.total ?? project.Total ?? 0,
  }));

  return {
    ...data,
    top_projects: topProjects,
    visits_30d: (data.visits_30d || []).map((row) => ({
      date: row.date,
      unique_visitors: row.unique_visitors ?? row.uniqueVisitors ?? 0,
      visits: row.visits,
    })),
  };
}
