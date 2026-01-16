import { createFileRoute } from "@tanstack/react-router";
import { MediaToolsPage } from "@/components/media-tools/MediaToolsPage";

export const Route = createFileRoute("/media-tools")({
  component: MediaToolsPage,
});
