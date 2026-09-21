<script setup lang="ts">
import { computed, onActivated, onMounted, onUnmounted, ref, watch } from "vue";
import { api, events } from "@/api";
import { useTransferStore } from "@/stores/transfer";
import { useToastStore } from "@/stores/toast";

type Status = "pending" | "success" | "failed";
interface SubmitItem {
  url: string;
  status: Status;
  reason?: string;
}
interface ItemUpdate {
  index: number;
  status: Status;
  reason?: string;
}

const transfer = useTransferStore();
const toast = useToastStore();

const paste = ref("");
const submitting = ref(false);
const progress = ref("");
const lastError = ref("");
const items = ref<SubmitItem[]>([]);

const success = computed(() => items.value.filter((i) => i.status === "success").length);
const failed = computed(() => items.value.filter((i) => i.status === "failed").length);
const pending = computed(() => items.value.filter((i) => i.status === "pending").length);
const urlCount = ref(0);

watch([items, paste], () => {
  const pasted = (paste.value || "").split("\n").map((l) => l.trim()).filter((l) => l.length > 0);
  urlCount.value = items.value.length + pasted.length;
}, { immediate: true, deep: true });

function startSubmit() {
  const lines = (paste.value || "").split("\n").map((l) => l.trim()).filter(Boolean);
  const existing = new Set(items.value.map((i) => i.url));
  for (const l of lines) {
    if (!existing.has(l)) {
      items.value.push({ url: l, status: "pending" });
      existing.add(l);
    }
  }
  paste.value = "";
  // 只提交未成功行：已成功跳过，避免重复消耗爬取配额
  const toSend = items.value.filter((i) => i.status !== "success").map((i) => i.url);
  if (!toSend.length) {
    toast.show("没有待提交的URL", "error");
    return;
  }
  submitting.value = true;
  lastError.value = "";
  api.startSubmit(toSend).then((err) => {
    if (err) {
      toast.show(err, "error");
      submitting.value = false;
    }
  });
}

function stopSubmit() {
  api.stopSubmit();
  submitting.value = false;
  progress.value = "已停止提交";
}

async function exportResults() {
  try {
    const err = await api.exportSubmit();
    if (err) toast.show(err, "error");
  } catch (e) {
    toast.show("导出失败: " + (e as Error).message, "error");
  }
}

async function importFile() {
  try {
    const urls = await api.importSubmitUrls();
    if (!urls || !urls.length) return;
    const cur = paste.value.trim();
    paste.value = cur ? cur + "\n" + urls.join("\n") : urls.join("\n");
    toast.show(`导入 ${urls.length} 个URL`);
  } catch (e) {
    toast.show("导入失败: " + (e as Error).message, "error");
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
  events.on("submit:start", (data) => {
    // 兼容后端 Start{urls} 对象与纯数组两种形状；合并更新不整表重置，
    // 名单行置 pending、新 URL 追加，已成功且不在名单中的行予以保留。
    const raw = data as unknown;
    const urls: string[] = Array.isArray(raw)
      ? (raw as string[])
      : ((raw as { urls?: string[]; URLs?: string[] })?.urls ??
        (raw as { urls?: string[]; URLs?: string[] })?.URLs ?? []);
    if (!urls.length) return;
    const indexOf = new Map(items.value.map((it, i) => [it.url, i]));
    const seen = new Set<string>();
    for (const u of urls) {
      if (seen.has(u)) continue;
      seen.add(u);
      const at = indexOf.get(u);
      if (at !== undefined) {
        items.value[at].status = "pending";
        items.value[at].reason = undefined;
      } else {
        indexOf.set(u, items.value.length);
        items.value.push({ url: u, status: "pending" });
      }
    }
  });
  events.on("submit:progress", (data) => {
    const p = data as { host: string; done: number; total: number; batch?: number; batches?: number };
    const batch = p.batches && p.batches > 1 ? ` [分批 ${p.batch}/${p.batches}]` : "";
    progress.value = `提交中 [${p.done}/${p.total}] (${p.host})${batch}`;
  });
  events.on("submit:item", (data) => {
    const u = data as ItemUpdate;
    if (u.index < items.value.length && items.value[u.index]) {
      items.value[u.index].status = u.status;
      items.value[u.index].reason = u.reason || undefined;
    }
  });
  events.on("submit:done", (data) => {
    const d = data as { success: number; failed: number; pending: number; error?: string };
    lastError.value = d.error || "";
    progress.value = "提交完成！";
    submitting.value = false;
    if (d.failed > 0) {
      toast.show(
        `提交完成！成功 ${d.success}，失败 ${d.failed}${d.error ? "：" + d.error : ""}`,
        "error",
      );
    } else {
      toast.show("提交完成！", "success");
    }
  });
  events.on("submit:error", (data) => {
    submitting.value = false;
    toast.show("提交出错: " + String(data), "error");
  });
});

onUnmounted(() => {
  events.off("submit:start");
  events.off("submit:progress");
  events.off("submit:item");
  events.off("submit:done");
  events.off("submit:error");
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
        <button class="btn" :class="{ 'btn-danger': submitting }" @click="submitting ? stopSubmit() : startSubmit()">
          {{ submitting ? "⏹ 停止" : "🚀 开始提交" }}
        </button>
        <button class="btn-outlined" @click="importFile">⬆ 导入URL</button>
        <button class="btn-outlined" @click="exportResults">⬇ 导出结果</button>
        <span class="count-pill">当前列表 <b>{{ items.length + (paste.trim() ? paste.trim().split("\n").filter(l => l.trim()).length : 0) }}</b></span>
      </div>
      <div class="hint-callout">
        <span class="hint-icon">ℹ</span>
        <span class="hint-text">单批最多 <b>10,000</b> 条 URL（IndexNow 官方上限），超出将自动分批提交；触发限流时按服务器要求退避重试。已成功行再次提交会自动跳过，避免重复消耗爬取配额。</span>
      </div>
      <p class="progress-text">{{ progress }}</p>
      <p v-if="lastError" class="error-text">{{ lastError }}</p>
    </div>

    <div class="stats-section">
      <div class="stats">
        <div class="stat-card success">
          <span class="stat-icon">✓</span>
          <span class="stat-label">成功</span>
          <span class="stat-value">{{ success }}</span>
        </div>
        <div class="stat-card danger">
          <span class="stat-icon">✕</span>
          <span class="stat-label">失败</span>
          <span class="stat-value">{{ failed }}</span>
        </div>
        <div class="stat-card muted">
          <span class="stat-icon">⏳</span>
          <span class="stat-label">待提交</span>
          <span class="stat-value">{{ pending }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-section {
  padding: 16px 18px;
  border: 1px solid var(--color-border-soft);
  border-radius: var(--radius);
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #fff;
  box-shadow: var(--shadow-sm);
}
.textarea {
  resize: vertical;
  font-family: inherit;
}
.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.count-pill {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border-radius: 999px;
  background: var(--color-primary-bg);
  color: var(--color-primary);
  font-size: 13px;
  font-weight: 500;
}
.count-pill b {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}
.hint-callout {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 8px;
  padding: 10px 14px;
  border-radius: var(--radius);
  background: var(--color-primary-bg-soft);
  border-left: 3px solid var(--color-primary);
}
.hint-icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  margin-top: 1px;
}
.hint-text {
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}
.hint-text b {
  color: var(--color-primary);
  font-weight: 600;
}
.stats-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  flex: 1;
  padding-top: 16px;
}
.stats {
  display: flex;
  gap: 24px;
}
.stat-card {
  width: 132px;
  padding: 24px 16px;
  border-radius: var(--radius-lg);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: #fff;
  border: 1px solid var(--color-border-soft);
  box-shadow: var(--shadow-sm);
  transition: transform var(--transition), box-shadow var(--transition);
}
.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}
.stat-icon {
  font-size: 30px;
}
.stat-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}
.stat-value {
  font-size: 30px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.success .stat-icon,
.success .stat-label,
.success .stat-value {
  color: var(--color-success);
}
.danger .stat-icon,
.danger .stat-label,
.danger .stat-value {
  color: var(--color-danger);
}
.muted .stat-icon,
.muted .stat-label,
.muted .stat-value {
  color: var(--color-disabled-text);
}
</style>
