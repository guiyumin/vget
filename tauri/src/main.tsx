import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { invoke } from "@tauri-apps/api/core";
import "./index.css";
import { routeTree } from "./routeTree.gen";

// Apply theme on startup from config
invoke<{ theme: string }>("get_config")
  .then((config) => {
    const theme = config.theme || "light";
    if (theme === "dark") {
      document.documentElement.classList.add("dark");
    } else if (theme === "system") {
      const isDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      document.documentElement.classList.toggle("dark", isDark);
    }
  })
  .catch(() => {
    // Config not available yet, default to light
  });

const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
