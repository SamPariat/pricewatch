import Link from "next/link";
import { CheckCircle2, XCircle, Clock } from "lucide-react";
import { api } from "@/lib/api";
import { watchTitle } from "@/lib/format";
import type { DigestRun, Watch } from "@/lib/types";

export default async function RunsPage() {
  const [runs, watches] = await Promise.all([api.getRuns(), api.getWatches()]);
  const watchByID = new Map(watches.map((w) => [w.id, w]));

  const groups = groupByDay(runs);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-2xl md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Run history</h1>
        <p className="text-sm text-muted-foreground">Every fetch, in order — success or failure.</p>
      </div>

      {runs.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border py-16 text-center text-sm text-muted-foreground">
          No runs yet.
        </p>
      ) : (
        Object.entries(groups).map(([day, dayRuns]) => (
          <div key={day} className="flex flex-col gap-2.5">
            <div className="text-[12.5px] font-medium text-muted-foreground">{day}</div>
            {dayRuns.map((run) => (
              <RunRow key={run.run_id} run={run} watch={watchByID.get(run.watch_id)} />
            ))}
          </div>
        ))
      )}
    </div>
  );
}

function RunRow({ run, watch }: { run: DigestRun; watch?: Watch }) {
  const Icon = run.status === "success" ? CheckCircle2 : run.status === "failed" ? XCircle : Clock;
  const color = run.status === "success" ? "text-success" : run.status === "failed" ? "text-destructive" : "text-muted-foreground";

  return (
    <Link
      href={`/runs/${run.run_id}`}
      className="flex items-center gap-3 rounded-xl border border-border bg-card p-3.5 shadow-sm"
    >
      <Icon className={`size-[19px] shrink-0 ${color}`} />
      <div className="flex flex-1 flex-col">
        <span className="text-[14px] font-semibold">{watch ? watchTitle(watch) : run.watch_id}</span>
        <span className="font-mono text-[11.5px] text-muted-foreground">
          {new Date(run.started_at).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
          {run.status === "failed" && run.error ? ` · ${run.error}` : ""}
        </span>
      </div>
    </Link>
  );
}

function groupByDay(runs: DigestRun[]): Record<string, DigestRun[]> {
  const today = new Date().toDateString();
  const yesterday = new Date(Date.now() - 86_400_000).toDateString();

  const groups: Record<string, DigestRun[]> = {};
  for (const run of runs) {
    const d = new Date(run.started_at).toDateString();
    const label = d === today ? "Today" : d === yesterday ? "Yesterday" : d;
    (groups[label] ??= []).push(run);
  }
  return groups;
}
