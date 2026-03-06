import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { ThemeProvider } from "next-themes";

import "./index.css";
import App from "./App.tsx";
import { EnabledModulesProvider } from "@/state/enabled-modules-context";
import { setupGlobalUnauthorizedInterceptor } from "@/lib/admin-auth";

setupGlobalUnauthorizedInterceptor();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem>
      <BrowserRouter>
        <EnabledModulesProvider>
          <App />
        </EnabledModulesProvider>
      </BrowserRouter>
    </ThemeProvider>
  </StrictMode>,
);
