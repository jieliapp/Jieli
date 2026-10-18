/// <reference types="@sveltejs/kit" />

import type { ServerConfig } from '$lib/api/auth';

declare global {
  namespace App {
    interface Error {
      message: string;
    }
  }

  interface Window {
    __APP_CONFIG__?: ServerConfig;
  }
}

export {};
