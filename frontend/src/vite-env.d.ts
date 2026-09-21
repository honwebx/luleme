/// <reference types="vite/client" />

declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

interface Window {
  // Populated by Wails v2 at runtime (frontend/wailsjs + injected runtime).
  go?: {
    main?: {
      App?: {
        [key: string]: (...args: any[]) => Promise<any>;
      };
    };
  };
  runtime?: {
    EventsOn: (name: string, cb: (...args: any[]) => void) => void;
    EventsOff: (name: string) => void;
  };
}
