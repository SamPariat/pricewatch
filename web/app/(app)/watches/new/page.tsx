import { WatchForm } from "@/components/watch-form";

export default function NewWatchPage() {
  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">New watch</h1>
        <p className="text-sm text-muted-foreground">Track a flight, hotel, or rental&apos;s price over time.</p>
      </div>
      <WatchForm />
    </div>
  );
}
