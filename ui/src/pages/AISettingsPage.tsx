import { useApp } from "../context/AppContext";
import { AISettings } from "../components/AISettings";

export function AISettingsPage() {
  const { t, isConnected } = useApp();

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-4">
      <h1 className="text-xl font-medium text-zinc-900 dark:text-white">
        {t.ai_settings}
      </h1>
      <AISettings isConnected={isConnected} />
    </div>
  );
}
