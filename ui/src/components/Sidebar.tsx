import { Link, useLocation } from "@tanstack/react-router";
import clsx from "clsx";

import { FaDownload, FaGear, FaTruck, FaLayerGroup, FaMagnet } from "react-icons/fa6";
import { useApp } from "../context/AppContext";

interface SidebarProps {
  lang: string;
}

interface NavItem {
  to: string;
  icon: React.ReactNode;
  label: string;
  show?: boolean;
}

export function Sidebar({ lang }: SidebarProps) {
  const location = useLocation();
  const { t } = useApp();

  const navItems: NavItem[] = [
    {
      to: "/",
      icon: <FaDownload />,
      label: t.download,
      show: true,
    },
    {
      to: "/bulk",
      icon: <FaLayerGroup />,
      label: t.bulk_download,
      show: true,
    },
    {
      to: "/torrent",
      icon: <FaMagnet />,
      label: t.torrent,
      show: true,
    },
    {
      to: "/config",
      icon: <FaGear />,
      label: t.settings,
      show: true,
    },
    {
      to: "/kuaidi100",
      icon: <FaTruck />,
      label: "快递查询",
      show: lang === "zh",
    },
  ];

  const visibleItems = navItems.filter((item) => item.show !== false);

  return (
    <aside
      className={clsx(
        "flex flex-col h-full bg-white dark:bg-zinc-900 border-r border-zinc-300 dark:border-zinc-700 transition-all duration-300",
        "w-48"
      )}
    >
      <div className="flex-1 py-4">
        <nav className="flex flex-col gap-1 px-2">
          {visibleItems.map((item) => {
            const isActive =
              item.to === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.to);

            return (
              <Link
                key={item.to}
                to={item.to}
                className={clsx(
                  "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors",
                  isActive
                    ? "bg-blue-100 dark:bg-blue-900/50 text-blue-600 dark:text-blue-400 font-medium"
                    : "text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800"
                )}
              >
                <span className="text-lg">{item.icon}</span>
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
