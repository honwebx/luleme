<script setup lang="ts">
import { computed } from "vue";
import { useToastStore } from "@/stores/toast";

const toast = useToastStore();
const visible = computed(() => toast.message.length > 0);
const isError = computed(() => toast.type === "error");
</script>

<template>
  <transition name="toast">
    <div v-if="visible" class="toast" :class="isError ? 'error' : 'success'">
      <span class="icon">{{ isError ? "✕" : "✓" }}</span>
      <span class="text">{{ toast.message }}</span>
    </div>
  </transition>
</template>

<style scoped>
.toast {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  min-width: 220px;
  max-width: 80%;
  padding: 14px 24px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #fff;
  font-size: 15px;
  font-weight: 500;
  z-index: 9999;
  box-shadow: 0 6px 28px rgba(0, 0, 0, 0.18);
  pointer-events: none;
}

.toast.success {
  background: var(--color-success);
}

.toast.error {
  background: var(--color-danger);
}

.icon {
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}

.text {
  text-align: center;
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translate(-50%, -50%) scale(0.92);
}
</style>
