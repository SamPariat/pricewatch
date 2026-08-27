import { api } from "@/lib/container";
import { SettingsForm } from "@/components/settings-form";
import { getTranslations } from "next-intl/server";

export default async function SettingsPage() {
  const [settings, t] = await Promise.all([api.getSettings(), getTranslations("settings")]);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-lg md:p-8">
      <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
      <SettingsForm settings={settings} />
    </div>
  );
}
