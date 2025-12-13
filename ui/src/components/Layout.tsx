import { Outlet } from "@tanstack/react-router";
import clsx from "clsx";
import { CiLight, CiDark } from "react-icons/ci";
import { Sidebar } from "./Sidebar";
import { useApp } from "../context/AppContext";
import logo from "../assets/logo.png";

export function Layout() {
  const { health, isConnected, darkMode, setDarkMode, configLang } = useApp();

  return (
    <div className="flex w-full h-screen max-w-4xl bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white transition-colors">
      <Sidebar lang={configLang} />

      <div className="flex-1 flex flex-col overflow-hidden">
        <header className="flex justify-between items-center px-6 py-3 bg-white dark:bg-zinc-900 border-b border-zinc-300 dark:border-zinc-700">
          <div className="flex items-center gap-3">
            <img
              src={logo}
              alt="vget"
              className={clsx(
                "w-8 h-8 object-contain transition-all",
                !isConnected && "grayscale opacity-50"
              )}
            />
            <h1 className="text-xl font-bold bg-linear-to-br from-amber-400 to-orange-500 bg-clip-text text-transparent">
              VGet Server
            </h1>
          </div>
          <div className="flex items-center gap-3">
            <button
              className="bg-transparent border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1.5 cursor-pointer text-base leading-none transition-colors hover:border-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
              onClick={() => setDarkMode(!darkMode)}
              title={darkMode ? "Switch to light mode" : "Switch to dark mode"}
            >
              {darkMode ? <CiLight /> : <CiDark />}
            </button>
            <span className="text-zinc-400 dark:text-zinc-600 text-sm px-2 py-1 bg-zinc-100 dark:bg-zinc-800 rounded">
              {health?.version || "..."}
            </span>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
