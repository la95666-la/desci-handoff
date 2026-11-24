import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';

const routes: RouteRecordRaw[] = [
  {
    component: BasicLayout,
    meta: {
      icon: 'lucide:layers',
      order: 0,
      title: 'JoinCare',
    },
    name: 'JoinCare',
    path: '/joincare',
    redirect: '/joincare/users',
    children: [
      // ========== 用户 & 权限 ==========
      {
        name: 'JoinCareGroupUsers',
        path: '/joincare/group-users',
        meta: {
          icon: 'lucide:user-cog',
          title: '用户 & 权限',
        },
        redirect: '/joincare/users',
        children: [
          {
            name: 'JoinCareUsers',
            path: '/joincare/users',
            component: () => import('#/views/joincare/users/index.vue'),
            meta: {
              icon: 'lucide:users',
              title: '用户管理',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareLoginLogs',
            path: '/joincare/login-logs',
            component: () => import('#/views/joincare/login_logs/index.vue'),
            meta: {
              icon: 'lucide:log-in',
              title: '用户登录记录',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareAdminUsers',
            path: '/joincare/admin-users',
            component: () => import('#/views/joincare/admin_users/index.vue'),
            meta: {
              icon: 'lucide:user-cog',
              title: '管理员账号',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareRoles',
            path: '/joincare/roles',
            component: () => import('#/views/joincare/roles/index.vue'),
            meta: {
              icon: 'lucide:key-round',
              title: '角色权限',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareApprovals',
            path: '/joincare/approvals',
            component: () => import('#/views/joincare/approvals/index.vue'),
            meta: {
              icon: 'lucide:gavel',
              title: '4 眼审批',
              keepAlive: true,
            },
          },
        ],
      },

      // ========== 内容运营 ==========
      {
        name: 'JoinCareGroupContent',
        path: '/joincare/group-content',
        meta: {
          icon: 'lucide:file-text',
          title: '内容运营',
        },
        redirect: '/joincare/news',
        children: [
          {
            name: 'JoinCareNews',
            path: '/joincare/news',
            component: () => import('#/views/joincare/news/index.vue'),
            meta: {
              icon: 'lucide:newspaper',
              title: '新闻管理',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareBanner',
            path: '/joincare/banner',
            component: () => import('#/views/joincare/banner/index.vue'),
            meta: {
              icon: 'lucide:image-play',
              title: '首页 Banner',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareAnnouncement',
            path: '/joincare/announcement',
            component: () => import('#/views/joincare/announcement/index.vue'),
            meta: {
              icon: 'lucide:megaphone',
              title: '系统公告',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareCopywriting',
            path: '/joincare/copywriting',
            component: () => import('#/views/joincare/copywriting/index.vue'),
            meta: {
              icon: 'lucide:languages',
              title: 'i18n 文案',
              keepAlive: true,
            },
          },
        ],
      },

      // ========== Launchpad ==========
      {
        name: 'JoinCareGroupLaunchpad',
        path: '/joincare/group-launchpad',
        meta: {
          icon: 'lucide:rocket',
          title: 'Launchpad',
        },
        redirect: '/joincare/projects/list',
        children: [
          {
            name: 'JoinCareApplications',
            path: '/joincare/applications',
            component: () => import('#/views/joincare/applications/index.vue'),
            meta: {
              icon: 'lucide:file-check-2',
              title: '项目申请',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareProjectList',
            path: '/joincare/projects/list',
            component: () => import('#/views/joincare/projects/list.vue'),
            meta: {
              icon: 'lucide:list',
              title: '项目列表',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareV3TokenAdmission',
            path: '/joincare/ido-v3/token-admission',
            component: () =>
              import('#/views/joincare/v3_token_admission/index.vue'),
            meta: {
              icon: 'lucide:shield-check',
              title: 'V3 代币准入',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareChainTransactions',
            path: '/joincare/chain-transactions',
            component: () =>
              import('#/views/joincare/chain_transactions/index.vue'),
            meta: {
              icon: 'lucide:workflow',
              title: '链交易运维',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareChainEvents',
            path: '/joincare/chain-events',
            component: () => import('#/views/joincare/chain_events/index.vue'),
            meta: {
              icon: 'lucide:activity',
              title: '链事件运维',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareFundLedger',
            path: '/joincare/fund-ledger',
            meta: {
              icon: 'lucide:book-open-check',
              title: '资金账本',
            },
            redirect: '/joincare/fund-ledger/transactions',
            children: [
              {
                name: 'JoinCareFundLedgerTransactions',
                path: '/joincare/fund-ledger/transactions',
                component: () =>
                  import('#/views/joincare/fund_ledger/transactions.vue'),
                meta: {
                  icon: 'lucide:book-open',
                  title: '用户认购及权益及逐笔交易账本',
                  keepAlive: true,
                },
              },
              {
                name: 'JoinCareFundLedgerReconciliation',
                path: '/joincare/fund-ledger/reconciliation',
                component: () =>
                  import('#/views/joincare/fund_ledger/reconciliation.vue'),
                meta: {
                  icon: 'lucide:scale',
                  title: '项目资金身份及账本状态、对账检查',
                  keepAlive: true,
                },
              },
              {
                name: 'JoinCareFundLedgerAudit',
                path: '/joincare/fund-ledger/audit',
                component: () =>
                  import('#/views/joincare/fund_ledger/audit.vue'),
                meta: {
                  icon: 'lucide:shield-check',
                  title: '审计相关',
                  keepAlive: true,
                },
              },
            ],
          },
          {
            name: 'JoinCareProjectContent',
            path: '/joincare/projects/content',
            component: () =>
              import('#/views/joincare/projects/content_editor.vue'),
            meta: {
              icon: 'lucide:file-edit',
              title: '项目内容',
              keepAlive: true,
              // 项目内容编辑要 ?id=X 才能用，从列表进；隐藏侧栏入口避免误点
              hideInMenu: true,
            },
          },
          {
            name: 'JoinCareProjectOnchainParams',
            path: '/joincare/projects/:id/onchain-params',
            component: () =>
              import('#/views/joincare/projects/onchain_params.vue'),
            meta: {
              icon: 'lucide:settings-2',
              title: '链上参数',
              hideInMenu: true,
            },
          },
          {
            name: 'JoinCareV3PolicyPreview',
            path: '/joincare/projects/:id/ido-v3/policy-preview',
            component: () =>
              import('#/views/joincare/projects/v3_policy_preview.vue'),
            meta: { title: 'V3 名单预览（仅诊断）', hideInMenu: true },
          },
          {
            name: 'JoinCareFairLaunchSet',
            path: '/joincare/projects/:id/fair-launch/set',
            component: () =>
              import('#/views/joincare/projects/fair_launch_set.vue'),
            meta: {
              icon: 'lucide:badge-check',
              title: 'Fair Launch Set 只读摘要',
              hideInMenu: true,
            },
          },
          {
            name: 'JoinCareFairLaunchOperations',
            path: '/joincare/projects/:id/fair-launch/operations',
            component: () =>
              import('#/views/joincare/projects/fair_launch_operations.vue'),
            meta: {
              icon: 'lucide:shield-alert',
              title: 'Fair Launch 控制台',
              hideInMenu: true,
            },
          },
          {
            name: 'JoinCareFairLaunchExecutionQueue',
            path: '/joincare/projects/:id/fair-launch/execution-queue',
            component: () =>
              import('#/views/joincare/projects/fair_launch_execution_queue.vue'),
            meta: {
              icon: 'lucide:user-check',
              title: 'Fair Launch 第三执行队列',
              hideInMenu: true,
            },
          },
          {
            name: 'JoinCareProjectUpdates',
            path: '/joincare/projects/updates',
            component: () => import('#/views/joincare/projects/updates.vue'),
            meta: {
              icon: 'lucide:megaphone',
              title: '项目 Updates',
              keepAlive: true,
              hideInMenu: true,
            },
          },
        ],
      },

      // ========== 任务 & 积分 ==========
      {
        name: 'JoinCareGroupTasks',
        path: '/joincare/group-tasks',
        meta: {
          icon: 'lucide:list-checks',
          title: '任务 & 积分',
        },
        redirect: '/joincare/tasks',
        children: [
          {
            name: 'JoinCareTasks',
            path: '/joincare/tasks',
            component: () => import('#/views/joincare/tasks/index.vue'),
            meta: {
              icon: 'lucide:list-checks',
              title: '任务模板',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareCheckin',
            path: '/joincare/checkin',
            component: () => import('#/views/joincare/checkin/index.vue'),
            meta: {
              icon: 'lucide:calendar-check',
              title: '签到奖励设置',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCarePointFlows',
            path: '/joincare/point-flows',
            component: () => import('#/views/joincare/point_flows/index.vue'),
            meta: {
              icon: 'lucide:coins',
              title: '积分流水',
              keepAlive: true,
            },
          },
          {
            name: 'JoinCareTgActivity',
            path: '/joincare/tg-activity',
            component: () => import('#/views/joincare/tg_activity/index.vue'),
            meta: {
              icon: 'lucide:message-square',
              title: 'TG 活跃监控',
              keepAlive: true,
            },
          },
        ],
      },

      // ========== 系统 ==========
      {
        name: 'JoinCareGroupSystem',
        path: '/joincare/group-system',
        meta: {
          icon: 'lucide:shield-check',
          title: '系统',
        },
        redirect: '/joincare/audit-log',
        children: [
          {
            name: 'JoinCareAuditLog',
            path: '/joincare/audit-log',
            component: () => import('#/views/joincare/audit_log/index.vue'),
            meta: {
              icon: 'lucide:shield-check',
              title: '操作审计',
              keepAlive: true,
            },
          },
        ],
      },
    ],
  },
];

export default routes;
