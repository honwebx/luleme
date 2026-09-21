<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import logoUrl from "@/assets/logo.png";

interface MenuItem {
  route: string;
  title: string;
  icon: string;
  activeIcon: string;
}

const route = useRoute();
const router = useRouter();

const items: MenuItem[] = [
  { route: "/url_crawler", title: "网址", icon: "🌐", activeIcon: "🌐" },
  { route: "/url_checker", title: "查询", icon: "🔍", activeIcon: "🔍" },
  { route: "/url_submitter", title: "提交", icon: "📤", activeIcon: "📤" },
  { route: "/settings", title: "设置", icon: "⚙", activeIcon: "⚙" },
];

const activeRoute = computed(() =>
  route.path && route.path !== "/" ? route.path : "/url_crawler"
);

function navigate(r: string): void {
  router.push(r);
}
</script>

<template>
  <aside class="sidebar">
    <div class="brand">
      <img :src="logoUrl" class="logo" alt="录了么" />
      <span class="brand-name">录了么</span>
    </div>
    <div class="separator"></div>
    <div class="menu">
      <button
        v-for="item in items"
        :key="item.route"
        class="menu-item"
        :class="{ active: activeRoute === item.route }"
        @click="navigate(item.route)"
      >
        <span class="menu-icon">{{ item.icon }}</span>
        <span class="menu-title">{{ item.title }}</span>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-width);
  background: var(--color-sidebar-bg);
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-shrink: 0;
  height: 100%;
}

.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  padding: 20px 0 16px;
}

.logo {
  width: 38px;
  height: 38px;
  border-radius: var(--radius);
  object-fit: contain;
  display: block;
}

.brand-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: 1px;
  margin-top: -6px;
}

.separator {
  width: 60px;
  height: 1px;
  background: var(--color-border);
  margin: 0 10px;
}

.menu {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding-top: 10px;
}

.menu-item {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: var(--radius);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: var(--color-text-secondary);
  background: transparent;
  transition: background var(--transition), color var(--transition);
}

.menu-item:hover {
  color: var(--color-primary);
  background: var(--color-primary-bg-soft);
}

.menu-item.active {
  background: var(--color-primary-bg);
  color: var(--color-primary);
}

.menu-item.active::before {
  content: "";
  position: absolute;
  left: -8px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 24px;
  background: var(--color-primary);
  border-radius: 0 3px 3px 0;
}

.menu-icon {
  font-size: 22px;
  line-height: 1;
  transition: transform var(--transition);
}

.menu-item:hover .menu-icon,
.menu-item.active .menu-icon {
  transform: scale(1.08);
}

.menu-title {
  font-size: 13px;
  font-weight: 500;
}
</style>
