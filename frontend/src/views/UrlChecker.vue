<script setup lang="ts">
import { computed, nextTick, onActivated, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, events } from "@/api";
import { useTransferStore } from "@/stores/transfer";
import { useToastStore } from "@/stores/toast";

type Status = "pending" | "indexed" | "not_indexed" | "error";
interface CheckItem {
  url: string;
  status: Status;
}
interface ItemUpdate {
  index: number;
  status: Status;
}

const router = useRouter();
const transfer = useTransferStore();
const toast = useToastStore();

const paste = ref("");
const checking = ref(false);
const progress = ref("");
const items = ref<CheckItem[]>([]);
const tableBody = ref<HTMLElement>();
const rowEls = ref<HTMLElement[]>([]);
// 本轮下标→URL 映射：后端 check:item 的 index 是本轮子集坐标，
// 必须翻译成 URL 再落行，直接按下标写全表会在跳过已收录时串行。
let runUrls: string[] = [];

function setRowEl(idx: number, el: unknown) {
  if (el) rowEls.value[idx] = el as HTMLElement;
}

function scrollToRow(idx: number) {
  nextTick(() => {
    rowEls.value[idx]?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  });
}

const notIndexed = computed(() => items.value.filter((i) => i.status === "not_indexed").map((i) => i.url));

function statusColor(s: Status): string {
  if (s === "indexed") return "var(--color-success)";
  if (s === "not_indexed") return "var(--color-danger)";
  if (s === "error") return "var(--color-warning)";
  return "var(--color-disabled-text)";
}
function statusBadge(s: Status): string {
  if (s === "indexed") return "badge badge-success";
  if (s === "not_indexed") return "badge badge-danger";
  if (s === "error") return "badge badge-warning";
  return "badge badge-muted";
}
function statusText(s: Status): string {
  if (s === "indexed") return "已收录";
  if (s === "not_indexed") return "未收录";
  if (s === "error") return "查询失败";
  return "待查询";
}

function startCheck() {
  if (checking.value) return;
  const lines = (paste.value || "").split("\n").map((l) => l.trim()).filter(Boolean);
  if (items.value.length === 0 && lines.length === 0) {
    toast.show("请先添加待查询URL", "error");
    return;
  }
  // append new lines to existing list (dedupe)
  const existing = new Set(items.value.map((i) => i.url));
  for (const l of lines) {
    if (!existing.has(l)) {
      items.value.push({ url: l, status: "pending" });
      existing.add(l);
    }
  }
  paste.value = "";
  // 只重查未定论的行：已收录跳过，未收录/查询失败/待查询参与本轮
  const recheck = items.value.filter((i) => i.status !== "indexed");
  if (recheck.length === 0) {
    toast.show("当前列表均已收录，无需重查");
    return;
  }
  // 立刻翻成待查询，让用户看得出本轮在查哪些；后端拒收时再回滚
  const prev = new Map(recheck.map((i) => [i.url, i.status] as [string, Status]));
  for (const it of recheck) it.status = "pending";
  checking.value = true;
  api.startCheck(recheck.map((i) => i.url)).then((err) => {
    if (err) {
      toast.show(err, "error");
      for (const it of recheck) it.status = prev.get(it.url) ?? it.status;
      checking.value = false;
    }
  });
}

function stopCheck() {
  // 只发停止信号，不翻按钮状态：等后端真正退出并发出 check:done 后，
  // 由 check:done 处理器统一翻转，避免“停止中又点开始”造成双任务互串。
  api.stopCheck();
  progress.value = "正在停止…";
}

async function importFile() {
  try {
    const urls = await api.importCheckUrls();
    if (!urls || !urls.length) return;
    const cur = paste.value.trim();
    paste.value = cur ? cur + "\n" + urls.join("\n") : urls.join("\n");
    toast.show(`成功导入 ${urls.length} 个URL`);
  } catch (e) {
    toast.show("导入失败: " + (e as Error).message, "error");
  }
}

function clearList() {
  if (checking.value) {
    toast.show("查询进行中，请先停止后再清空", "error");
    return;
  }
  if (!items.value.length) return;
  const n = items.value.length;
  items.value = [];
  rowEls.value = [];
  runUrls = [];
  progress.value = "";
  toast.show(`已清空 ${n} 个URL`);
}

function sendToSubmitter() {
  if (!notIndexed.value.length) return;
  transfer.setUrls(notIndexed.value);
  router.push("/url_submitter");
  toast.show(`已发送 ${notIndexed.value.length} 个URL到提交列表`);
}

async function exportResults() {
  try {
    const err = await api.exportCheck();
    if (err) toast.show("导出失败: " + err, "error");
    else toast.show("导出成功");
  } catch (e) {
    toast.show("导出失败: " + (e as Error).message, "error");
  }
}

// consume URLs on activation (works under keep-alive, mirrors did_mount)
onActivated(() => {
  const incoming = transfer.takeUrls();
  if (incoming.length) {
    const cur = paste.value.trim();
    paste.value = cur ? cur + "\n" + incoming.join("\n") : incoming.join("\n");
  }
});

onMounted(() => {
  events.on("check:start", (data) => {
    // 合并更新：只动本轮名单里的行（置 pending、新 URL 追加），
    // 本轮跳过的已收录行保持原样，避免整表重置洗掉它们。
    // rowEls 不动：存量行下标不变、引用继续有效，追加行由 ref 回调自行登记。
    // 注意：后端发的是 Start{URLs} 结构体，前端收到的是 {urls:[...]} 对象而非数组。
    const raw = data as unknown;
    const urls: string[] = Array.isArray(raw)
      ? (raw as string[])
      : Array.isArray((raw as { urls?: unknown })?.urls)
        ? (raw as { urls: string[] }).urls
        : [];
    runUrls = [...urls];
    for (const u of urls) {
      const ex = items.value.find((i) => i.url === u);
      if (ex) ex.status = "pending";
      else items.value.push({ url: u, status: "pending" as Status });
    }
    const first = urls.length ? items.value.findIndex((i) => i.url === urls[0]) : -1;
    if (first >= 0) scrollToRow(first);
  });
  events.on("check:progress", (data) => {
    const p = data as { index: number; total: number; url: string };
    progress.value = `查询中 [${p.index}/${p.total}]: ${p.url.slice(0, 40)}...`;
  });
  events.on("check:item", (data) => {
    const u = data as ItemUpdate;
    // 按 URL 落行：u.index 是本轮子集下标，先翻译成 URL 再找行；
    // 对不上（过期/串扰事件）直接丢弃，绝不按下标硬写。
    const url = runUrls[u.index];
    if (url == null) return;
    const rowIdx = items.value.findIndex((i) => i.url === url);
    if (rowIdx < 0) return;
    items.value[rowIdx].status = u.status;
    // 滚到下一条待查询；已到末尾则停在最后一条
    let target = rowIdx;
    for (let i = rowIdx + 1; i < items.value.length; i++) {
      if (items.value[i].status === "pending") {
        target = i;
        break;
      }
    }
    scrollToRow(target);
  });
  events.on("check:done", (data) => {
    const d = data as { indexed: number; notIndexed: number };
    progress.value = `本轮查询完成，共 ${d.indexed} 个已收录`;
    // 先提示条数，再翻按钮状态（与 stopCheck 只发信号、不预翻的约定配套）
    toast.show(`本轮查询完成！共 ${d.indexed} 个已收录，${d.notIndexed} 个未收录`);
    checking.value = false;
    runUrls = [];
  });
  events.on("check:error", (data) => {
    // 导入失败事件，不代表任务结束，不翻按钮状态
    toast.show("查询出错: " + String(data), "error");
  });
});

onUnmounted(() => {
  events.off("check:start");
  events.off("check:progress");
  events.off("check:item");
  events.off("check:done");
  events.off("check:error");
});
</script>

<template>
  <div class="content-area">
    <div class="input-section">
      <textarea
        v-model="paste"
        class="input textarea"
        rows="4"
        placeholder="https://example.com&#10;https://example2.com"
      ></textarea>
      <div class="actions">
        <button class="btn" :class="{ 'btn-danger': checking }" @click="checking ? stopCheck() : startCheck()">
          {{ checking ? "⏹ 停止查询" : "🔍 开始查询" }}
        </button>
        <button class="btn-outlined" @click="importFile">⬆ 导入URL</button>
      </div>
      <p class="progress-text">{{ progress }}</p>
    </div>

    <div class="result-section">
      <div class="result-head">
        <div class="head-left">
          <span class="page-title">查询结果</span>
          <span class="count">当前列表: {{ items.length }} 个URL</span>
        </div>
        <div class="head-actions">
          <button class="btn" :disabled="!notIndexed.length" @click="sendToSubmitter">↗ 发送到提交</button>
          <button class="btn" :disabled="!items.length" @click="exportResults">⬇ 导出结果</button>
        </div>
      </div>
      <div class="panel">
        <div class="table-header">
          <span class="col-status">状态</span>
          <span class="col-url">URL</span>
          <button class="clear-link" :disabled="checking || !items.length" @click="clearList" title="清空下方结果列表">
            🗑 清空列表
          </button>
        </div>
        <div ref="tableBody" class="table-body">
          <div v-for="(it, idx) in items" :key="idx" :ref="(el) => setRowEl(idx, el)" class="table-row">
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
.textarea {
  resize: vertical;
  font-family: inherit;
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
  align-items: center;
}
.clear-link {
  background: none;
  border: none;
  padding: 4px 2px;
  font-size: 13px;
  font-weight: 400;
  color: var(--color-text-muted);
  cursor: pointer;
  white-space: nowrap;
}
.clear-link:hover:not(:disabled) {
  color: var(--color-danger);
}
.clear-link:disabled {
  color: var(--color-disabled-text);
  cursor: not-allowed;
  text-decoration: none;
}
.table-header {
  display: flex;
  gap: 5px;
  align-items: center;
  background: var(--color-header);
  padding: 10px 14px;
  font-weight: 700;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.col-status {
  width: 88px;
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
