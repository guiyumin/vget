import { useState } from "react";
import { Outlet } from "@tanstack/react-router";
import clsx from "clsx";
import { CiLight, CiDark } from "react-icons/ci";
import { FaBars, FaTimes } from "react-icons/fa";
import { Sidebar } from "./Sidebar";
import { useApp } from "../context/AppContext";
import logo from "../assets/logo.png";

export function Layout() {
  const { health, isConnected, darkMode, setDarkMode, configLang } = useApp();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="flex w-full h-screen max-w-4xl bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white transition-colors">
      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar - hidden on mobile by default, shown when sidebarOpen */}
      <div
        className={clsx(
          "fixed md:relative z-50 md:z-auto h-full transition-transform duration-300 md:transition-none",
          sidebarOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"
        )}
      >
        <Sidebar lang={configLang} onClose={() => setSidebarOpen(false)} />
      </div>

      <div className="flex-1 flex flex-col overflow-hidden">
        <header className="flex justify-between items-center px-4 md:px-6 py-3 bg-white dark:bg-zinc-900 border-b border-zinc-300 dark:border-zinc-700">
          <div className="flex items-center gap-3">
            {/* Mobile menu button */}
            <button
              className="md:hidden bg-transparent border border-zinc-300 dark:border-zinc-700 rounded-md p-2 cursor-pointer text-base leading-none transition-colors hover:border-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
              onClick={() => setSidebarOpen(!sidebarOpen)}
              aria-label="Toggle menu"
            >
              {sidebarOpen ? <FaTimes /> : <FaBars />}
            </button>
            <img
              src={logo}
              alt="vget"
              className={clsx(
                "w-8 h-8 object-contain transition-all",
                !isConnected && "grayscale opacity-50"
              )}
            />
            <h1 className="text-lg md:text-xl font-bold bg-linear-to-br from-amber-400 to-orange-500 bg-clip-text text-transparent">
              VGet Server
            </h1>
          </div>
          <div className="flex items-center gap-2 md:gap-3">
            <button
              className="bg-transparent border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1.5 cursor-pointer text-base leading-none transition-colors hover:border-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
              onClick={() => setDarkMode(!darkMode)}
              title={darkMode ? "Switch to light mode" : "Switch to dark mode"}
            >
              {darkMode ? <CiLight /> : <CiDark />}
            </button>
            <span className="text-zinc-400 dark:text-zinc-600 text-xs md:text-sm px-2 py-1 bg-zinc-100 dark:bg-zinc-800 rounded">
              {health?.version || "..."}
            </span>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
