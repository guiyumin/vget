import { Link, useLocation } from "@tanstack/react-router";
import clsx from "clsx";

import {
  FaDownload,
  FaGear,
  FaTruck,
  FaLayerGroup,
  FaMagnet,
  FaCloud,
  FaPodcast,
  FaB,
  FaWandMagicSparkles,
  FaMicrophone,
} from "react-icons/fa6";
import { useApp } from "../context/AppContext";

interface SidebarProps {
  lang: string;
}

interface NavItem {
  to?: string;
  icon: React.ReactNode;
  label: string;
  show?: boolean;
  children?: NavItem[];
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
      icon: <FaWandMagicSparkles />,
      label: t.ai,
      show: true,
      children: [
        {
          to: "/ai/speech-to-text",
          icon: <FaMicrophone />,
          label: t.ai_speech_to_text,
          show: true,
        },
        {
          to: "/ai/settings",
          icon: <FaGear />,
          label: t.settings,
          show: true,
        },
      ],
    },
    {
      to: "/bilibili",
      icon: <FaB />,
      label: "哔哩哔哩",
      show: lang === "zh",
    },

    {
      to: "/podcast",
      icon: <FaPodcast />,
      label: t.podcast,
      show: true,
    },
    {
      to: "/webdav",
      icon: <FaCloud />,
      label: t.webdav_browser,
      show: true,
    },
    {
      to: "/torrent",
      icon: <FaMagnet />,
      label: t.torrent,
      show: true,
    },
    {
      to: "/kuaidi100",
      icon: <FaTruck />,
      label: "快递查询",
      show: lang === "zh",
    },
    {
      to: "/config",
      icon: <FaGear />,
      label: t.settings,
      show: true,
    },
  ];

  const visibleItems = navItems.filter((item) => item.show !== false);

  const renderNavItem = (item: NavItem, isChild = false) => {
    const hasChildren = item.children && item.children.length > 0;
    const visibleChildren =
      item.children?.filter((c) => c.show !== false) ?? [];

    // Check if this item or any child is active
    const isActive = item.to
      ? item.to === "/"
        ? location.pathname === "/"
        : location.pathname.startsWith(item.to)
      : false;
    const hasActiveChild = visibleChildren.some(
      (child) => child.to && location.pathname.startsWith(child.to)
    );

    if (hasChildren) {
      // Always expanded section (non-collapsible)
      return (
        <div key={item.label}>
          <div
            className={clsx(
              "flex items-center gap-3 px-3 py-2.5 text-sm",
              hasActiveChild
                ? "text-blue-600 dark:text-blue-400 font-medium"
                : "text-zinc-600 dark:text-zinc-400"
            )}
          >
            <span className="text-lg">{item.icon}</span>
            <span>{item.label}</span>
          </div>
          <div className="ml-4 mt-1 flex flex-col gap-1">
            {visibleChildren.map((child) => renderNavItem(child, true))}
          </div>
        </div>
      );
    }

    return (
      <Link
        key={item.to}
        to={item.to!}
        className={clsx(
          "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors",
          isChild && "pl-4",
          isActive
            ? "bg-blue-100 dark:bg-blue-900/50 text-blue-600 dark:text-blue-400 font-medium"
            : "text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800"
        )}
      >
        <span className="text-lg">{item.icon}</span>
        <span>{item.label}</span>
      </Link>
    );
  };

  return (
    <aside
      className={clsx(
        "flex flex-col h-full bg-white dark:bg-zinc-900 border-r border-zinc-300 dark:border-zinc-700 transition-all duration-300",
        "w-48"
      )}
    >
      <div className="flex-1 py-4">
        <nav className="flex flex-col gap-1 px-2">
          {visibleItems.map((item) => renderNavItem(item))}
        </nav>
      </div>
    </aside>
  );
}
