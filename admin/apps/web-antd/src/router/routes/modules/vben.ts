import type { RouteRecordRaw } from 'vue-router';

// Profile / 安全设置 / MFA 是 vben 自带 demo（硬编码假数据、假编辑、假 MFA），
// backend 无对应 API，暂时下掉 topbar 入口和路由。
// 之后要接后端 profile / 改密 / MFA 时再恢复：
//   1. 恢复 basic.vue 里 menus 里的 Profile 项
//   2. 恢复本文件里的 Profile 路由
const routes: RouteRecordRaw[] = [];

export default routes;
