import { useApp } from "../context/AppContext";
import { Torrent } from "../components/Torrent";

export function TorrentPage() {
  const { isConnected, torrentEnabled } = useApp();

  return (
    <div className="max-w-3xl mx-auto">
      <Torrent isConnected={isConnected} torrentEnabled={torrentEnabled} />
    </div>
  );
}
