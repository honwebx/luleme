<script setup lang="ts">
import Sidebar from "@/components/Sidebar.vue";
import Toast from "@/components/Toast.vue";
</script>

<template>
  <div class="layout">
    <Sidebar />
    <div class="divider"></div>
    <main class="content">
      <router-view v-slot="{ Component }">
        <keep-alive>
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </main>
    <Toast />
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100%;
  width: 100%;
}

.divider {
  width: 1px;
  background: var(--color-border);
  flex-shrink: 0;
}

.content {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #ffffff;
  overflow: hidden;
}
</style>
