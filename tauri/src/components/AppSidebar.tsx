import { Link, useLocation } from "@tanstack/react-router";
import { Download, Settings, ChevronLeft, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import logo from "@/assets/logo.png";

interface NavItem {
  to: string;
  icon: React.ReactNode;
  label: string;
}

const navItems: NavItem[] = [
  {
    to: "/",
    icon: <Download className="h-5 w-5" />,
    label: "Download",
  },
  {
    to: "/media-tools",
    icon: <Wrench className="h-5 w-5" />,
    label: "Media Tools",
  },
  {
    to: "/settings",
    icon: <Settings className="h-5 w-5" />,
    label: "Settings",
  },
];

interface AppSidebarProps {
  collapsed: boolean;
  onToggle: () => void;
}

export function AppSidebar({ collapsed, onToggle }: AppSidebarProps) {
  const location = useLocation();

  const isActive = (path: string) => {
    if (path === "/") {
      return location.pathname === "/";
    }
    return location.pathname.startsWith(path);
  };

  return (
    <TooltipProvider delayDuration={0}>
      <aside
        className={cn(
          "flex flex-col h-full bg-muted/30 border-r transition-all duration-300",
          collapsed ? "w-16" : "w-48"
        )}
      >
        {/* Header with logo */}
        <div
          className={cn(
            "flex items-center border-b h-14",
            collapsed ? "justify-center px-2" : "justify-between px-3"
          )}
        >
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  onClick={onToggle}
                  className="flex items-center justify-center h-10 w-10 rounded-md hover:bg-muted transition-colors"
                >
                  <img src={logo} alt="VGet" className="h-8 w-8" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="right">Expand menu</TooltipContent>
            </Tooltip>
          ) : (
            <>
              <div className="flex items-center gap-2">
                <img src={logo} alt="VGet" className="h-8 w-8" />
                <span className="font-semibold text-lg">VGet</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={onToggle}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
            </>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 py-4">
          <ul className="space-y-1 px-2">
            {navItems.map((item) => {
              const active = isActive(item.to);

              if (collapsed) {
                return (
                  <li key={item.to}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Link
                          to={item.to}
                          className={cn(
                            "flex items-center justify-center h-10 w-full rounded-md transition-colors",
                            active
                              ? "bg-primary text-primary-foreground"
                              : "text-muted-foreground hover:bg-muted hover:text-foreground"
                          )}
                        >
                          {item.icon}
                        </Link>
                      </TooltipTrigger>
                      <TooltipContent side="right">{item.label}</TooltipContent>
                    </Tooltip>
                  </li>
                );
              }

              return (
                <li key={item.to}>
                  <Link
                    to={item.to}
                    className={cn(
                      "flex items-center gap-3 px-3 py-2.5 rounded-md text-sm transition-colors",
                      active
                        ? "bg-primary text-primary-foreground"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                  >
                    {item.icon}
                    <span>{item.label}</span>
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        {/* Footer */}
        <div className="p-2 border-t">
          {!collapsed && (
            <p className="text-xs text-muted-foreground px-2">VGet Desktop</p>
          )}
        </div>
      </aside>
    </TooltipProvider>
  );
}
