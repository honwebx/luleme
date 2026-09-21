<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { api, events } from "@/api";
import { useTransferStore } from "@/stores/transfer";
import { useToastStore } from "@/stores/toast";

interface CrawlItem {
  url: string;
  status: number;
}
interface CrawlDone {
  valid: number;
  total: number;
  error: string;
}

const router = useRouter();
const transfer = useTransferStore();
const toast = useToastStore();

const urlInput = ref("");
const crawling = ref(false);
const progress = ref("");
const items = ref<CrawlItem[]>([]);
const errorMsg = ref("");
const tableBody = ref<HTMLElement>();

watch(() => items.value.length, () => {
  nextTick(() => {
    if (tableBody.value) tableBody.value.scrollTop = tableBody.value.scrollHeight;
  });
});

const validUrls = computed(() => items.value.filter((i) => i.status === 200).map((i) => i.url));

function statusColor(s: number): string {
  if (s >= 200 && s < 300) return "var(--color-success)";
  if (s >= 300 && s < 400) return "var(--color-warning)";
  return "var(--color-danger)";
}
function statusBadge(s: number): string {
  if (s >= 200 && s < 300) return "badge badge-success";
  if (s >= 300 && s < 400) return "badge badge-warning";
  return "badge badge-danger";
}
function statusText(s: number): string {
  return s === 0 ? "错误" : String(s);
}

async function toggleCrawl() {
  if (crawling.value) {
    api.stopCrawl();
    crawling.value = false;
    progress.value = `已停止爬取！共 ${items.value.length} 个页面`;
    return;
  }
  const url = urlInput.value.trim();
  if (!url) {
    toast.show("请输入网站URL", "error");
    return;
  }
  items.value = [];
  progress.value = "";
  errorMsg.value = "";
  crawling.value = true;
  try {
    const err = await api.startCrawl(url);
    if (err) {
      toast.show(err, "error");
      crawling.value = false;
    }
  } catch (e) {
    toast.show((e as Error).message, "error");
    crawling.value = false;
  }
}

function sendToChecker() {
  if (!validUrls.value.length) return;
  transfer.setUrls(validUrls.value);
  router.push("/url_checker");
  toast.show(`已发送 ${validUrls.value.length} 个URL到查询`);
}

async function exportResults() {
  try {
    const err = await api.exportCrawl();
    if (err) toast.show("导出失败: " + err, "error");
    else toast.show("导出成功", "success");
  } catch (e) {
    toast.show("导出失败: " + (e as Error).message, "error");
  }
}

onMounted(() => {
  events.on("crawl:progress", (text) => {
    progress.value = String(text);
  });
  events.on("crawl:item", (data) => {
    items.value.push(data as CrawlItem);
  });
  events.on("crawl:done", (data) => {
    const d = data as CrawlDone;
    progress.value = `爬取完成！共发现 ${d.valid} 个有效页面`;
    crawling.value = false;
    if (d.valid > 0) toast.show(`爬取完成！共发现 ${d.valid} 个有效页面`, "success");
  });
  events.on("crawl:error", (data) => {
    errorMsg.value = String(data);
    crawling.value = false;
    toast.show("爬取出错: " + errorMsg.value, "error");
  });
});

onUnmounted(() => {
  events.off("crawl:progress");
  events.off("crawl:item");
  events.off("crawl:done");
  events.off("crawl:error");
});
</script>

<template>
  <div class="content-area">
    <div class="input-section">
      <input v-model="urlInput" class="input" placeholder="https://example.com" @keyup.enter="toggleCrawl" />
      <div class="actions">
        <button class="btn" :class="{ 'btn-danger': crawling }" @click="toggleCrawl">
          {{ crawling ? "⏹ 停止爬取" : "▶ 开始爬取" }}
        </button>
      </div>
      <p class="progress-text">{{ progress }}</p>
    </div>

    <div class="result-section">
      <div class="result-head">
        <div class="head-left">
          <span class="page-title">爬取结果</span>
          <span class="count">共 {{ items.length }} 个，有效 {{ validUrls.length }} 个</span>
        </div>
        <div class="head-actions">
          <button class="btn" :disabled="!validUrls.length" @click="sendToChecker">↗ 发送到查询</button>
          <button class="btn" :disabled="!items.length" @click="exportResults">⬇ 导出结果</button>
        </div>
      </div>
      <div class="panel">
        <div class="table-header">
          <span class="col-status">状态</span>
          <span class="col-url">URL</span>
        </div>
        <div ref="tableBody" class="table-body">
          <div v-for="(it, idx) in items" :key="idx" class="table-row">
            <span class="col-status"><span :class="statusBadge(it.status)">{{ statusText(it.status) }}</span></span>
            <span class="col-url">{{ it.url }}</span>
          </div>
          <p v-if="!items.length" class="empty">暂无数据</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.content-area {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.input-section {
  flex-shrink: 0;
  padding: 4px 0 8px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.actions {
  display: flex;
  gap: 10px;
}
.result-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
  min-height: 0;
}
.result-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.head-left {
  display: flex;
  gap: 15px;
  align-items: center;
}
.count {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-muted);
}
.head-actions {
  display: flex;
  gap: 10px;
}
.table-header {
  display: flex;
  gap: 5px;
  background: var(--color-header);
  padding: 10px 14px;
  font-weight: 700;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.col-status {
  width: 64px;
}
.col-url {
  flex: 1;
  word-break: break-all;
}
.table-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}
.table-row {
  display: flex;
  gap: 5px;
  padding: 8px 14px;
  font-size: 13px;
  border-bottom: 1px solid var(--color-border-soft);
  transition: background var(--transition);
}
.table-row:hover {
  background: var(--color-primary-bg-soft);
}
.empty {
  color: var(--color-disabled-text);
  text-align: center;
  padding: 48px;
  font-size: 14px;
}
.panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
</style>
