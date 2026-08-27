import { api } from "@/lib/container";
import { WatchForm } from "@/components/watch-form";
import { getTranslations } from "next-intl/server";

export default async function EditWatchPage(props: PageProps<"/watches/[id]/edit">) {
  const { id } = await props.params;
  const [watch, t] = await Promise.all([api.getWatch(id), getTranslations("editWatch")]);

  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
      </div>
      <WatchForm watch={watch} />
    </div>
  );
}
