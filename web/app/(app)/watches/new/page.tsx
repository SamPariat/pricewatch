import { WatchForm } from "@/components/watch-form";
import { getTranslations } from "next-intl/server";

export default async function NewWatchPage() {
  const t = await getTranslations("newWatch");
  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>
      <WatchForm />
    </div>
  );
}
