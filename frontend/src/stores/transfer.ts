import { defineStore } from "pinia";
import { ref } from "vue";

/**
 * Cross-view URL transfer (crawler -> checker -> submitter).
 * Mirrors the original UrlImportExport in-memory shuttle: a destination view
 * consumes URLs on mount via takeUrls(), which clears the buffer so a re-mount
 * does not re-fill (matches the original set_urls([]) contract).
 */
export const useTransferStore = defineStore("transfer", () => {
  const urls = ref<string[]>([]);

  function setUrls(list: string[]): void {
    urls.value = [...list];
  }

  /** Returns the buffered URLs and clears the buffer. */
  function takeUrls(): string[] {
    const out = [...urls.value];
    urls.value = [];
    return out;
  }

  function hasUrls(): boolean {
    return urls.value.length > 0;
  }

  return { urls, setUrls, takeUrls, hasUrls };
});
