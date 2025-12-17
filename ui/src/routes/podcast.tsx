import { createFileRoute } from "@tanstack/react-router";
import { PodcastPage } from "../pages/PodcastPage";

export const Route = createFileRoute("/podcast")({
  component: PodcastPage,
});
