import { createFileRoute } from "@tanstack/react-router";
import { Kuaidi100Page } from "../pages/Kuaidi100Page";

export const Route = createFileRoute("/kuaidi100")({
  component: Kuaidi100Page,
});
