import { createFileRoute } from "@tanstack/react-router";
import { SettingsPage } from "@/components/settings";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});
