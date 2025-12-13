import { createFileRoute } from "@tanstack/react-router";
import { ConfigPage } from "../pages/ConfigPage";

export const Route = createFileRoute("/config")({
  component: ConfigPage,
});
