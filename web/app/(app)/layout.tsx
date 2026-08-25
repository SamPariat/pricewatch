import Link from "next/link";
import { Plus, Send } from "lucide-react";
import { api } from "@/lib/container";
import { SidebarNav, BottomNav } from "@/components/nav-links";
import { Button } from "@/components/ui/button";
import { logout } from "@/app/login/actions";
import { cn } from "@/lib/utils";

async function ChannelPill() {
  let status: "linked" | "disconnected" = "disconnected";
  try {
    status = (await api.getChannelStatus()).status;
  } catch {
    // Backend unreachable or Telegram check failed — show disconnected
    // rather than crashing the whole shell over a status indicator.
  }
  return (
    <div className="flex items-center gap-2 rounded-md bg-sidebar-accent px-3 py-2.5">
      <span
        className={cn("size-1.5 shrink-0 rounded-full", status === "linked" ? "bg-success" : "bg-muted-foreground")}
      />
      <span className="text-xs font-medium">
        {status === "linked" ? "Telegram connected" : "Telegram disconnected"}
      </span>
    </div>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      {/* Desktop sidebar */}
      <aside className="hidden w-58 shrink-0 flex-col border-r border-sidebar-border bg-sidebar p-3.5 text-sidebar-foreground md:flex">
        <div className="flex items-center gap-2.5 px-2 pb-6 pt-1">
          <div className="flex size-7.5 items-center justify-center rounded-md bg-primary">
            <Send className="size-3.5 text-primary-foreground" />
          </div>
          <span className="text-[15px] font-bold tracking-tight">Pricewatch</span>
        </div>
        <SidebarNav />
        <div className="flex-1" />
        <ChannelPill />
        <form action={logout} className="mt-2">
          <Button type="submit" variant="ghost" size="sm" className="w-full justify-start text-muted-foreground">
            Sign out
          </Button>
        </form>
      </aside>

      {/* Mobile header */}
      <header className="flex items-center justify-between border-b border-border px-5 pb-3 pt-5 md:hidden">
        <span className="text-[15px] font-bold tracking-tight">Pricewatch</span>
        <Link
          href="/watches/new"
          className="flex size-8.5 items-center justify-center rounded-md bg-primary text-primary-foreground"
        >
          <Plus className="size-4" />
        </Link>
      </header>

      <main className="flex-1 pb-16 md:pb-0">{children}</main>

      {/* Mobile bottom nav */}
      <div className="fixed inset-x-0 bottom-0 md:hidden">
        <BottomNav />
      </div>
    </div>
  );
}
