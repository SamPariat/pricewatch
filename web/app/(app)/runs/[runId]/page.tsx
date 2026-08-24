import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { RunEvent } from "@/lib/types";

export default async function RunDetailPage(props: PageProps<"/runs/[runId]">) {
  const { runId } = await props.params;
  const events = await api.getRunEvents(runId);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-2xl md:p-8">
      <div className="flex items-center gap-3">
        <Link href="/runs" className="text-muted-foreground">
          <ArrowLeft className="size-4.5" />
        </Link>
        <div>
          <h1 className="text-lg font-bold tracking-tight">Run timeline</h1>
          <p className="font-mono text-[11.5px] text-muted-foreground">{runId}</p>
        </div>
      </div>

      {events.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border py-16 text-center text-sm text-muted-foreground">
          No events recorded for this run.
        </p>
      ) : (
        <div className="rounded-xl border border-border bg-card p-4">
          {events.map((event, i) => (
            <EventRow key={i} event={event} isLast={i === events.length - 1} />
          ))}
        </div>
      )}
    </div>
  );
}

function EventRow({ event, isLast }: { event: RunEvent; isLast: boolean }) {
  const dotColor =
    event.level === "error" ? "bg-destructive" : event.level === "warn" ? "bg-warning" : "bg-success";

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <span className={cn("size-2 shrink-0 rounded-full", dotColor)} />
        {!isLast && <span className="min-h-5.5 w-px flex-1 bg-border" />}
      </div>
      <div className="flex flex-1 items-center justify-between pb-4.5">
        <div className="flex flex-col gap-0.5">
          <span className="text-[12.5px] font-medium capitalize">{event.stage}</span>
          <span className="text-[12px] text-muted-foreground">{event.msg}</span>
        </div>
        <span className="shrink-0 pl-3 font-mono text-[11px] text-muted-foreground">
          {new Date(event.at).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
        </span>
      </div>
    </div>
  );
}
