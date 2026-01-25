import { useApp } from "../context/AppContext";
// import { Torrent } from "../components/Torrent";

export function TorrentPage() {
  // const { isConnected, torrentEnabled } = useApp();
  const { t } = useApp();

  // Coming Soon - uncomment below to enable torrent feature
  return (
    <div className="max-w-3xl mx-auto p-0">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6">{t.torrent}</h1>
      <div className="bg-zinc-100 dark:bg-zinc-800 rounded-lg p-6 sm:p-8 text-center">
        <p className="text-zinc-500 dark:text-zinc-400">{t.coming_soon}</p>
      </div>
    </div>
  );

  // return (
  //   <div className="max-w-3xl mx-auto">
  //     <Torrent isConnected={isConnected} torrentEnabled={torrentEnabled} />
  //   </div>
  // );
}
