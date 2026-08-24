import { api } from "@/lib/api";
import { SettingsForm } from "@/components/settings-form";

export default async function SettingsPage() {
  const settings = await api.getSettings();

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-lg md:p-8">
      <h1 className="text-xl font-bold tracking-tight">Settings</h1>
      <SettingsForm settings={settings} />
    </div>
  );
}
