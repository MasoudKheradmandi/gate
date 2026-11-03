import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The build is embedded into the Go binary (control/internal/webui/dist).
// In development, API calls are proxied to a running corvus-control (CORVUS_API).
const target = process.env.CORVUS_API ?? "https://localhost:9443";

export default defineConfig({
  plugins: [react()],
  build: { outDir: "../control/internal/webui/dist", emptyOutDir: true, sourcemap: false, chunkSizeWarningLimit: 900 },
  server: {
    proxy: {
      "/api": {
        target, secure: false, changeOrigin: true,
        // The backend rejects state-changing requests whose Origin host differs from Host.
        configure: (proxy) => {
          proxy.on("proxyReq", (req) => { if (req.getHeader("origin")) req.setHeader("origin", target); });
        },
      },
    },
  },
});
