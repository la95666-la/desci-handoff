<script lang="ts" setup>
import { computed } from 'vue';

import { AuthPageLayout } from '@vben/layouts';
import { preferences } from '@vben/preferences';

const logo = computed(() => preferences.logo.source);
</script>

<template>
  <AuthPageLayout
    app-name=""
    :logo="logo"
    page-description=""
    page-title=""
  >
    <!-- 左上角自定义 logo：宽度调大到 180px，不显示应用名 -->
    <template #logo>
      <div class="absolute top-0 left-0 z-10 flex flex-1">
        <div class="mt-4 ml-6 flex flex-1 items-center">
          <img
            v-if="logo"
            alt="JoinCare"
            :src="logo"
            width="220"
            style="height: auto"
          />
        </div>
      </div>
    </template>
  </AuthPageLayout>
</template>
