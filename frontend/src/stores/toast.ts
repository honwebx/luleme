import { defineStore } from "pinia";
import { ref } from "vue";

export type ToastType = "success" | "error";

/**
 * Global toast store. A single shared toast is rendered once in App.vue;
 * views call `show()` instead of maintaining their own toast state.
 */
export const useToastStore = defineStore("toast", () => {
  const message = ref("");
  const type = ref<ToastType>("success");
  let timer: number | undefined;

  function show(text: string, t: ToastType = "success", duration = 2500): void {
    message.value = text;
    type.value = t;
    if (timer !== undefined) window.clearTimeout(timer);
    timer = window.setTimeout(() => {
      message.value = "";
    }, duration);
  }

  function clear(): void {
    message.value = "";
    if (timer !== undefined) window.clearTimeout(timer);
  }

  return { message, type, show, clear };
});
