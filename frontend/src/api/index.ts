/**
 * Thin typed wrapper around the Wails-generated bindings.
 *
 * Go method bindings are imported from wailsjs/go/main/App (auto-generated).
 * Runtime events are imported from wailsjs/runtime/runtime (auto-generated).
 * Both are produced by `wails dev` / `wails build`.
 *
 * In a plain browser session (npm run dev without Wails) these modules
 * delegate to absent globals, so calls degrade to rejected promises /
 * no-ops to surface the missing backend.
 */

import {
  Version,
  BrowserMode,
  GetApiKey,
  SaveApiKey,
  DataDir,
  StartCrawl,
  StopCrawl,
  ExportCrawl,
  StartCheck,
  StopCheck,
  ImportCheckUrls,
  ExportCheck,
  StartSubmit,
  StopSubmit,
  ImportSubmitUrls,
  ExportSubmit,
} from "../../wailsjs/go/main/App";

import { EventsOn, EventsOff, BrowserOpenURL } from "../../wailsjs/runtime/runtime";

export const api = {
  version: () => Version(),
  browserMode: () => BrowserMode(),
  getApiKey: () => GetApiKey(),
  saveApiKey: (key: string) => SaveApiKey(key),
  dataDir: () => DataDir(),
  // crawler
  startCrawl: (url: string) => StartCrawl(url),
  stopCrawl: () => StopCrawl(),
  exportCrawl: () => ExportCrawl(),
  // checker
  startCheck: (urls: string[]) => StartCheck(urls),
  stopCheck: () => StopCheck(),
  importCheckUrls: () => ImportCheckUrls(),
  exportCheck: () => ExportCheck(),
  // submitter
  startSubmit: (urls: string[]) => StartSubmit(urls),
  stopSubmit: () => StopSubmit(),
  importSubmitUrls: () => ImportSubmitUrls(),
  exportSubmit: () => ExportSubmit(),
};

export const events = {
  on: (name: string, cb: (...args: unknown[]) => void) => EventsOn(name, cb),
  off: (name: string) => EventsOff(name),
};

/**
 * Open a URL in the user's default browser (not inside the app webview).
 * Falls back to window.open when the Wails runtime is absent (plain web dev).
 */
export function openExternal(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, "_blank", "noopener");
  }
}
