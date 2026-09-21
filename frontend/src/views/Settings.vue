<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, openExternal } from "@/api";
import { useToastStore } from "@/stores/toast";

const indexNowGuideUrl = "https://www.bing.com/indexnow/getstarted";

function openGuide() {
  openExternal(indexNowGuideUrl);
}

const apiKey = ref("");
const saving = ref(false);
const dataDir = ref("");
const browserMode = ref("");

const toast = useToastStore();
const httpOnly = computed(() => browserMode.value.includes("HTTP"));

onMounted(async () => {
  try {
    apiKey.value = await api.getApiKey();
  } catch (e) {
    toast.show((e as Error).message, "error");
  }
  try {
    dataDir.value = await api.dataDir();
  } catch {
    // non-critical
  }
  try {
    browserMode.value = await api.browserMode();
  } catch {
    // non-critical
  }
});

async function save() {
  saving.value = true;
  try {
    const errMsg = await api.saveApiKey(apiKey.value.trim());
    if (errMsg) {
      toast.show("保存失败: " + errMsg, "error");
    } else {
      toast.show("设置已保存", "success");
    }
  } catch (e) {
    toast.show("保存失败: " + (e as Error).message, "error");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="content-area">
    <div class="settings-intro">
      <h2 class="page-title">Bing IndexNow API 设置</h2>
      <div class="info-box">
        <div class="info-box-title">API Key 获取方法</div>
        <ol class="steps">
          <li class="step">访问 <a class="link link-external" @click.prevent="openGuide">{{ indexNowGuideUrl }}</a></li>
          <li class="step">按照指引验证网站所有权</li>
          <li class="step">获取 API Key 并在下方填写</li>
        </ol>
        <div class="info-note">
          <span class="info-note-icon">!</span>
          单批最多提交 <b>10,000</b> 条 URL（IndexNow 官方上限）。
        </div>
      </div>
    </div>

    <div class="settings-form">
      <input
        v-model="apiKey"
        type="password"
        class="input"
        placeholder="请输入 IndexNow API Key"
      />
      <div>
        <button class="btn" :disabled="saving" @click="save">💾 保存设置</button>
      </div>
    </div>

    <div v-if="dataDir || browserMode" class="runtime-card">
      <div class="runtime-title">运行环境</div>
      <div v-if="dataDir" class="runtime-row">
        <span class="runtime-label">📁 配置目录</span>
        <span class="runtime-value mono">{{ dataDir }}</span>
      </div>
      <div v-if="browserMode" class="runtime-row">
        <span class="runtime-label">🌐 浏览器渲染</span>
        <span class="runtime-value" :class="{ warn: httpOnly }">
          <span class="status-dot" :class="httpOnly ? 'dot-warn' : 'dot-ok'"></span>
          {{ browserMode }}
          <span v-if="httpOnly" class="runtime-extra">SPA 爬取与 Bing 查询命中率下降</span>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.content-area {
  display: flex;
  flex-direction: column;
  gap: 15px;
}
.settings-intro {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.info-box {
  background: var(--color-header);
  padding: 16px 18px;
  border-radius: var(--radius);
  display: flex;
  flex-direction: column;
  gap: 12px;
  border: 1px solid var(--color-border-soft);
}
.info-box-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-text);
}
.steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.step {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  counter-increment: step;
}
.step::before {
  content: counter(step);
  flex-shrink: 0;
  width: 20px;
  height: 20px;
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
.steps {
  counter-reset: step;
}
.info-note {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius);
  background: var(--color-warning-bg);
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}
.info-note b {
  color: var(--color-warning);
  font-weight: 600;
}
.info-note-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--color-warning);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.link {
  color: var(--color-primary);
}
.link-external {
  cursor: pointer;
}
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.runtime-card {
  padding: 14px 18px;
  border-radius: var(--radius);
  border: 1px solid var(--color-border-soft);
  background: var(--color-header);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.runtime-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-text-secondary);
  padding-bottom: 8px;
  border-bottom: 1px solid var(--color-border-soft);
}
.runtime-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.runtime-label {
  flex-shrink: 0;
  width: 100px;
  font-size: 13px;
  color: var(--color-text-muted);
}
.runtime-value {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--color-text);
  word-break: break-all;
}
.runtime-value.mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.runtime-value.warn {
  color: var(--color-warning);
  font-weight: 500;
}
.runtime-extra {
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 400;
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot-ok {
  background: var(--color-success);
  box-shadow: 0 0 0 3px var(--color-success-bg);
}
.dot-warn {
  background: var(--color-warning);
  box-shadow: 0 0 0 3px var(--color-warning-bg);
}
</style>
