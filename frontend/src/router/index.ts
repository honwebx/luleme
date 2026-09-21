import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";

const routes: RouteRecordRaw[] = [
  {
    path: "/url_crawler",
    name: "crawler",
    component: () => import("@/views/UrlCrawler.vue"),
  },
  {
    path: "/url_checker",
    name: "checker",
    component: () => import("@/views/UrlChecker.vue"),
  },
  {
    path: "/url_submitter",
    name: "submitter",
    component: () => import("@/views/UrlSubmitter.vue"),
  },
  {
    path: "/settings",
    name: "settings",
    component: () => import("@/views/Settings.vue"),
  },
  { path: "/", redirect: "/url_crawler" },
  { path: "/:pathMatch(.*)*", redirect: "/url_crawler" },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
});

export default router;
