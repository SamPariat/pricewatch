import { api } from "@/lib/api";
import { WatchForm } from "@/components/watch-form";

export default async function EditWatchPage(props: PageProps<"/watches/[id]/edit">) {
  const { id } = await props.params;
  const watch = await api.getWatch(id);

  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">Edit watch</h1>
      </div>
      <WatchForm watch={watch} />
    </div>
  );
}
