import { createFileRoute } from "@tanstack/react-router";
import { SpeechToTextPage } from "../../pages/SpeechToTextPage";

export const Route = createFileRoute("/ai/speech-to-text")({
  component: SpeechToTextPage,
});
