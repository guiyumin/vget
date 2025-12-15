import { useApp } from "../context/AppContext";
import { ConfigEditor, type ConfigValues } from "../components/ConfigEditor";
import { TorrentSettings } from "../components/TorrentSettings";

export function ConfigPage() {
  const {
    isConnected,
    t,
    serverT,
    configExists,
    configLang,
    configFormat,
    configQuality,
    serverPort,
    maxConcurrent,
    apiKey,
    kuaidi100Key,
    kuaidi100Customer,
    webdavServers,
    saveConfig,
    addWebDAV,
    deleteWebDAV,
  } = useApp();

  const handleSaveConfig = async (values: ConfigValues) => {
    await saveConfig(values);
  };

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-4">
      {!configExists && (
        <div className="flex items-start gap-3 p-3 bg-amber-100 dark:bg-amber-900 border border-amber-500 rounded-lg">
          <span className="text-xl leading-none">⚠️</span>
          <div className="flex-1">
            <p className="text-amber-800 dark:text-amber-100 text-sm">
              {serverT.no_config_warning}
            </p>
            <p className="text-amber-700 dark:text-amber-200 text-xs mt-1 opacity-80">
              {serverT.run_init_hint}
            </p>
          </div>
        </div>
      )}

      <ConfigEditor
        isConnected={isConnected}
        t={t}
        initialLang={configLang}
        initialFormat={configFormat}
        initialQuality={configQuality}
        initialMaxConcurrent={maxConcurrent}
        initialApiKey={apiKey}
        initialKuaidi100Key={kuaidi100Key}
        initialKuaidi100Customer={kuaidi100Customer}
        serverPort={serverPort}
        webdavServers={webdavServers}
        onSave={handleSaveConfig}
        onCancel={() => {}}
        onAddWebDAV={addWebDAV}
        onDeleteWebDAV={deleteWebDAV}
      />

      <TorrentSettings isConnected={isConnected} />
    </div>
  );
}
