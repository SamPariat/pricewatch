import Link from "next/link";
import { CheckCircle2, XCircle, Clock } from "lucide-react";
import { api } from "@/lib/container";
import { watchTitle } from "@/lib/format";
import { getLocale, getTranslations } from "next-intl/server";
import type { DigestRun, Watch } from "@/lib/types";

type RunsT = Awaited<ReturnType<typeof getTranslations<"runs">>>;
type WatchFormT = Awaited<ReturnType<typeof getTranslations<"watchForm">>>;

export default async function RunsPage() {
  const [runs, watches, locale, t, tForm] = await Promise.all([
    api.getRuns(),
    api.getWatches(),
    getLocale(),
    getTranslations("runs"),
    getTranslations("watchForm"),
  ]);
  const watchByID = new Map(watches.map((w) => [w.id, w]));

  const groups = groupByDay(runs, t);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-2xl md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {runs.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border py-16 text-center text-sm text-muted-foreground">
          {t("empty")}
        </p>
      ) : (
        Object.entries(groups).map(([day, dayRuns]) => (
          <div key={day} className="flex flex-col gap-2.5">
            <div className="text-[12.5px] font-medium text-muted-foreground">{day}</div>
            {dayRuns.map((run) => (
              <RunRow key={run.run_id} run={run} watch={watchByID.get(run.watch_id)} locale={locale} tForm={tForm} />
            ))}
          </div>
        ))
      )}
    </div>
  );
}

function RunRow({ run, watch, locale, tForm }: { run: DigestRun; watch?: Watch; locale: string; tForm: WatchFormT }) {
  const Icon = run.status === "success" ? CheckCircle2 : run.status === "failed" ? XCircle : Clock;
  const color = run.status === "success" ? "text-success" : run.status === "failed" ? "text-destructive" : "text-muted-foreground";
  const timeTag = locale === "hi" ? "hi-IN" : "en-US";

  return (
    <Link
      href={`/runs/${run.run_id}`}
      className="flex items-center gap-3 rounded-xl border border-border bg-card p-3.5 shadow-sm"
    >
      <Icon className={`size-[19px] shrink-0 ${color}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[14px] font-semibold">{watch ? watchTitle(watch, tForm) : run.watch_id}</span>
        <span className="font-mono text-[11.5px] text-muted-foreground">
          {new Date(run.started_at).toLocaleTimeString(timeTag, { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
        </span>
        {/* Full error lives on the run-detail timeline this row links to
            (RunDetailPage) — a raw Go error string, often a long
            URL-bearing wrapped error, was dumped in full here before and
            it made the list unreadable. One clamped line is enough to
            recognize "which run failed and roughly why" at a glance. */}
        {run.status === "failed" && run.error && (
          <span className="line-clamp-1 text-[11.5px] break-all text-destructive/80">{run.error}</span>
        )}
      </div>
    </Link>
  );
}

function groupByDay(runs: DigestRun[], t: RunsT): Record<string, DigestRun[]> {
  const today = new Date().toDateString();
  const yesterday = new Date(Date.now() - 86_400_000).toDateString();

  const groups: Record<string, DigestRun[]> = {};
  for (const run of runs) {
    const d = new Date(run.started_at).toDateString();
    const label = d === today ? t("today") : d === yesterday ? t("yesterday") : d;
    (groups[label] ??= []).push(run);
  }
  return groups;
}
