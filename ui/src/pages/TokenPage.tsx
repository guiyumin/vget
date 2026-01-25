import { useState } from "react";
import { useApp } from "../context/AppContext";

interface GenerateTokenResponse {
  code: number;
  data: { jwt: string } | null;
  message: string;
}

export function TokenPage() {
  const { isConnected, t } = useApp();
  const [payload, setPayload] = useState("{}");
  const [payloadError, setPayloadError] = useState<string | null>(null);
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [copied, setCopied] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const validatePayload = (value: string): boolean => {
    try {
      JSON.parse(value);
      setPayloadError(null);
      return true;
    } catch {
      setPayloadError(t.token_invalid_json);
      return false;
    }
  };

  const handlePayloadChange = (value: string) => {
    setPayload(value);
    if (value.trim()) {
      validatePayload(value);
    } else {
      setPayloadError(null);
    }
  };

  const handleGenerate = async () => {
    setGenerating(true);
    setErrorMessage(null);
    setGeneratedToken(null);

    try {
      let parsedPayload = {};
      if (payload.trim()) {
        if (!validatePayload(payload)) {
          setGenerating(false);
          return;
        }
        parsedPayload = JSON.parse(payload);
      }

      const res = await fetch("/api/auth/token", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ payload: parsedPayload }),
      });

      const data: GenerateTokenResponse = await res.json();

      if (data.code === 201 && data.data?.jwt) {
        setGeneratedToken(data.data.jwt);
      } else {
        setErrorMessage(data.message);
      }
    } catch (err) {
      setErrorMessage("Failed to generate token");
    } finally {
      setGenerating(false);
    }
  };

  const handleCopy = async () => {
    if (generatedToken) {
      await navigator.clipboard.writeText(generatedToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <div className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg p-4 sm:p-6">
        <h1 className="text-base sm:text-lg font-semibold text-zinc-900 dark:text-white mb-4">
          {t.token_title}
        </h1>

        <p className="text-xs sm:text-sm text-zinc-600 dark:text-zinc-400 mb-6">
          {t.token_description}
        </p>

        {/* Custom Payload */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">
            {t.token_custom_payload}
          </label>
          <textarea
            className={`w-full h-24 sm:h-32 px-3 py-2 font-mono text-xs sm:text-sm border rounded-lg bg-zinc-50 dark:bg-zinc-950 text-zinc-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 ${
              payloadError
                ? "border-red-500"
                : "border-zinc-300 dark:border-zinc-700"
            }`}
            placeholder='{"user": "john", "scope": "read"}'
            value={payload}
            onChange={(e) => handlePayloadChange(e.target.value)}
            disabled={!isConnected || generating}
          />
          {payloadError && (
            <p className="mt-1 text-xs text-red-500">{payloadError}</p>
          )}
          <p className="mt-1 text-xs text-zinc-500">
            {t.token_custom_payload_hint}
          </p>
        </div>

        {/* Generate Button */}
        <button
          className="w-full px-4 py-2.5 rounded-lg text-sm font-medium cursor-pointer transition-colors bg-blue-500 text-white hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
          onClick={handleGenerate}
          disabled={!isConnected || generating || !!payloadError}
        >
          {generating ? t.token_generating : t.token_generate}
        </button>

        {/* Error Message */}
        {errorMessage && (
          <div className="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
            <p className="text-sm text-red-600 dark:text-red-400">
              {errorMessage}
            </p>
          </div>
        )}

        {/* Generated Token */}
        {generatedToken && (
          <div className="mt-6 p-4 bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-700 rounded-lg">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
                {t.token_generated}
              </span>
              <button
                className="px-3 py-1 rounded text-xs font-medium cursor-pointer transition-colors bg-zinc-200 dark:bg-zinc-800 hover:bg-zinc-300 dark:hover:bg-zinc-700 text-zinc-700 dark:text-zinc-300"
                onClick={handleCopy}
              >
                {copied ? t.token_copied : t.token_copy}
              </button>
            </div>
            <code className="block w-full p-3 bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-700 rounded text-xs font-mono text-zinc-800 dark:text-zinc-200 break-all select-all overflow-x-auto">
              {generatedToken}
            </code>
            <div className="mt-3 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded">
              <p className="text-xs text-blue-700 dark:text-blue-300 font-medium mb-1">
                {t.token_usage}:
              </p>
              <code className="text-xs text-blue-600 dark:text-blue-400 font-mono">
                Authorization: Bearer {generatedToken.slice(0, 20)}...
              </code>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
