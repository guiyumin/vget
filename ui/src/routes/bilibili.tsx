import { createFileRoute } from "@tanstack/react-router";
import { BilibiliPage } from "../pages/BilibiliPage";

export const Route = createFileRoute("/bilibili")({
  component: BilibiliPage,
});
