import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return;
          }

          if (id.includes("recharts")) return "vendor-charts";
          if (id.includes("radix-ui") || id.includes("@base-ui")) return "vendor-radix";
          if (id.includes("react-router-dom")) return "vendor-routing";
          if (id.includes("sonner") || id.includes("lucide-react")) return "vendor-ui";
          if (id.includes("axios")) return "vendor-axios";

          return "vendor-misc";
        },
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
