"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutList, History, Settings } from "lucide-react";
import { cn } from "@/lib/utils";

const LINKS = [
  { href: "/watches", label: "Watches", icon: LayoutList },
  { href: "/runs", label: "Runs", icon: History },
  { href: "/settings", label: "Settings", icon: Settings },
];

export function SidebarNav() {
  const pathname = usePathname();
  return (
    <nav className="flex flex-col gap-0.5">
      {LINKS.map(({ href, label, icon: Icon }) => {
        const active = pathname.startsWith(href);
        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "bg-sidebar-accent text-sidebar-accent-foreground"
                : "text-sidebar-foreground hover:bg-sidebar-accent/60",
            )}
          >
            <Icon className="size-4" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}

export function BottomNav() {
  const pathname = usePathname();
  return (
    <nav className="flex border-t border-border bg-card px-2 pt-2.5 pb-3.5">
      {LINKS.map(({ href, label, icon: Icon }) => {
        const active = pathname.startsWith(href);
        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "flex flex-1 flex-col items-center gap-0.5",
              active ? "text-foreground" : "text-muted-foreground",
            )}
          >
            <Icon className="size-5" />
            <span className={cn("text-[10.5px]", active ? "font-semibold" : "font-medium")}>{label}</span>
          </Link>
        );
      })}
    </nav>
  );
}
