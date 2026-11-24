<script lang="ts" setup>
import type { DashboardOverview } from '#/api/joincare/dashboard';

import { computed, onMounted, ref } from 'vue';

import { EchartsUI, useEcharts } from '@vben/plugins/echarts';

import { Card, Empty, message, Spin, Table, Tag } from 'ant-design-vue';

import { getDashboardOverview } from '#/api/joincare/dashboard';

const loading = ref(true);
const data = ref<DashboardOverview | null>(null);

async function load() {
  loading.value = true;
  try {
    data.value = await getDashboardOverview();
  } catch (error: any) {
    message.error(error?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

// ===== Overview 卡片 =====
const overviewCards = computed(() => {
  const d = data.value;
  if (!d) return [];
  return [
    {
      title: '总用户量',
      value: d.user.total,
      hint: `今日新增 +${d.user.new_today} · 7 日新增 +${d.user.new_7d}`,
      icon: 'lucide:users',
      color: '#1677ff',
    },
    {
      title: '项目总数',
      value: d.project.total,
      hint: `Pre-launch ${d.project.pre_launch} · Warmup ${d.project.warmup} · Active ${d.project.active}`,
      icon: 'lucide:rocket',
      color: '#52c41a',
    },
    {
      title: 'Launchpad 订阅数',
      value: d.launchpad.subscribers,
      hint: `关注 ${d.launchpad.watches} · 白名单 ${d.launchpad.whitelist}`,
      icon: 'lucide:bell',
      color: '#722ed1',
    },
    {
      title: '今日签到',
      value: d.checkin.today,
      hint: `TG 今日消息 ${d.tg.messages_today} · 活跃用户 ${d.tg.active_users_today}`,
      icon: 'lucide:calendar-check',
      color: '#fa8c16',
    },
  ];
});

// ===== 用户增长折线图 =====
const growthChartRef = ref();
const { renderEcharts: renderGrowth } = useEcharts(growthChartRef);

function drawGrowth() {
  const d = data.value;
  if (!d) return;
  renderGrowth({
    tooltip: { trigger: 'axis' },
    grid: { top: 30, right: 20, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: d.user_growth_30d.map((r) => r.date.slice(5)),
      axisLabel: { fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
    },
    series: [
      {
        name: '每日新增用户',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: '#1677ff', width: 2 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(22, 119, 255, 0.35)' },
              { offset: 1, color: 'rgba(22, 119, 255, 0)' },
            ],
          },
        },
        data: d.user_growth_30d.map((r) => r.new_users),
      },
    ],
  });
}

// ===== C 端每日访问量折线图 =====
const visitsChartRef = ref();
const { renderEcharts: renderVisits } = useEcharts(visitsChartRef);

function drawVisits() {
  const d = data.value;
  if (!d) return;
  renderVisits({
    tooltip: { trigger: 'axis' },
    legend: { data: ['页面访问量（PV）', '独立访客（UV）'], top: 0 },
    grid: { top: 45, right: 20, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: d.visits_30d.map((row) => row.date.slice(5)),
      axisLabel: { fontSize: 11 },
    },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: '页面访问量（PV）',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: '#13c2c2', width: 2 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(19, 194, 194, 0.35)' },
              { offset: 1, color: 'rgba(19, 194, 194, 0)' },
            ],
          },
        },
        data: d.visits_30d.map((row) => row.visits),
      },
      {
        name: '独立访客（UV）',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: '#722ed1', width: 2 },
        itemStyle: { color: '#722ed1' },
        data: d.visits_30d.map((row) => row.unique_visitors),
      },
    ],
  });
}

// ===== 项目状态分布饼图 =====
const projectChartRef = ref();
const { renderEcharts: renderProject } = useEcharts(projectChartRef);

function drawProject() {
  const d = data.value;
  if (!d) return;
  renderProject({
    tooltip: { trigger: 'item' },
    legend: {
      orient: 'vertical',
      right: 10,
      top: 'center',
      itemGap: 12,
      textStyle: { fontSize: 12 },
    },
    series: [
      {
        type: 'pie',
        radius: ['45%', '72%'],
        center: ['38%', '50%'],
        avoidLabelOverlap: true,
        label: { show: false },
        labelLine: { show: false },
        data: [
          {
            value: d.project.pre_launch,
            name: 'Pre-launch',
            itemStyle: { color: '#1677ff' },
          },
          {
            value: d.project.warmup,
            name: 'Warmup',
            itemStyle: { color: '#fa8c16' },
          },
          {
            value: d.project.active,
            name: 'Active',
            itemStyle: { color: '#52c41a' },
          },
          {
            value: d.project.ended,
            name: 'Ended / Archived',
            itemStyle: { color: '#8c8c8c' },
          },
        ].filter((x) => x.value > 0),
      },
    ],
  });
}

// ===== Launchpad engagement 柱状图 =====
const engagementChartRef = ref();
const { renderEcharts: renderEngagement } = useEcharts(engagementChartRef);

function drawEngagement() {
  const d = data.value;
  if (!d || d.top_projects.length === 0) return;
  const names = d.top_projects.map((p) => p.name || `#${p.project_id}`);
  renderEngagement({
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { data: ['订阅', '关注', '白名单'], top: 0 },
    grid: { top: 40, right: 20, bottom: 30, left: 60 },
    xAxis: {
      type: 'category',
      data: names,
      axisLabel: {
        fontSize: 11,
        interval: 0,
        rotate: names.length > 3 ? 15 : 0,
      },
    },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: '订阅',
        type: 'bar',
        stack: 'e',
        data: d.top_projects.map((p) => p.subscribers),
        itemStyle: { color: '#722ed1' },
      },
      {
        name: '关注',
        type: 'bar',
        stack: 'e',
        data: d.top_projects.map((p) => p.watches),
        itemStyle: { color: '#eb2f96' },
      },
      {
        name: '白名单',
        type: 'bar',
        stack: 'e',
        data: d.top_projects.map((p) => p.whitelist),
        itemStyle: { color: '#fa8c16' },
      },
    ],
  });
}

onMounted(async () => {
  await load();
  // 等 DOM 渲染
  setTimeout(() => {
    drawGrowth();
    drawVisits();
    drawProject();
    drawEngagement();
  }, 50);
});
</script>

<template>
  <div class="p-5">
    <Spin :spinning="loading">
      <!-- Overview 4 卡 -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card
          v-for="c in overviewCards"
          :key="c.title"
          class="overview-card"
          :bordered="false"
        >
          <div class="flex items-start justify-between">
            <div class="flex flex-col">
              <span class="text-xs text-[var(--ant-color-text-secondary)]">
                {{ c.title }}
              </span>
              <span
                class="mt-1 text-3xl font-bold tabular-nums"
                :style="{ color: c.color }"
              >
                {{ c.value }}
              </span>
              <span class="mt-2 text-xs text-[var(--ant-color-text-tertiary)]">
                {{ c.hint }}
              </span>
            </div>
            <span
              class="flex h-12 w-12 items-center justify-center rounded-lg"
              :style="{ background: `${c.color}20` }"
            >
              <span
                class="iconify text-2xl"
                :data-icon="c.icon"
                :style="{ color: c.color }"
              ></span>
            </span>
          </div>
        </Card>
      </div>

      <!-- 图表区 -->
      <div class="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-3">
        <Card
          title="用户增长 · 最近 30 天"
          :bordered="false"
          class="xl:col-span-2"
        >
          <EchartsUI ref="growthChartRef" height="320px" />
        </Card>
        <Card title="项目状态分布" :bordered="false">
          <EchartsUI ref="projectChartRef" height="320px" />
        </Card>
      </div>

      <Card
        title="C 端每日访问量（PV / UV）· 最近 30 天"
        :bordered="false"
        class="mt-4"
      >
        <EchartsUI ref="visitsChartRef" height="320px" />
      </Card>

      <div class="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-3">
        <Card
          title="Top 项目 · 订阅 / 关注 / 白名单"
          :bordered="false"
          class="xl:col-span-2"
        >
          <EchartsUI ref="engagementChartRef" height="320px" />
          <Empty
            v-if="data && !data.top_projects.length"
            description="暂无项目数据"
            class="py-6"
          />
        </Card>
        <Card title="Top 项目明细" :bordered="false">
          <Table
            v-if="data"
            :data-source="data.top_projects"
            :columns="[
              { title: '项目', dataIndex: 'name' },
              { title: '订阅', dataIndex: 'subscribers', width: 60 },
              { title: '关注', dataIndex: 'watches', width: 60 },
              { title: '白名单', dataIndex: 'whitelist', width: 60 },
            ]"
            :pagination="false"
            size="small"
            row-key="project_id"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'name'">
                <div class="flex flex-col">
                  <span style="font-weight: 600">
                    {{ record.name || `#${record.project_id}` }}
                  </span>
                  <span class="text-xs text-[var(--ant-color-text-tertiary)]">
                    合计 <Tag color="blue" class="ml-1">{{ record.total }}</Tag>
                  </span>
                </div>
              </template>
            </template>
          </Table>
        </Card>
      </div>
    </Spin>
  </div>
</template>

<style scoped>
.overview-card {
  border-radius: 12px;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease;
}

.overview-card:hover {
  box-shadow: 0 12px 24px -8px rgb(0 0 0 / 12%);
  transform: translateY(-2px);
}
</style>
