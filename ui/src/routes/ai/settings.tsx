import { createFileRoute } from "@tanstack/react-router";
import { AISettingsPage } from "../../pages/AISettingsPage";

export const Route = createFileRoute("/ai/settings")({
  component: AISettingsPage,
});
