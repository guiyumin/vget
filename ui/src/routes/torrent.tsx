import { createFileRoute } from "@tanstack/react-router";
import { TorrentPage } from "../pages/TorrentPage";

export const Route = createFileRoute("/torrent")({
  component: TorrentPage,
});
