import { useState, useEffect } from "react";
import { useApp } from "../context/AppContext";
import { setConfigValue } from "../utils/apis";

export function BilibiliPage() {
  const { isConnected } = useApp();
  const [cookie, setCookie] = useState("");
  const [savedCookie, setSavedCookie] = useState("");
  const [saving, setSaving] = useState(false);
  const [showInstructions, setShowInstructions] = useState(false);

  // Load saved cookie on mount
  useEffect(() => {
    fetch("/api/config")
      .then((res) => res.json())
      .then((data) => {
        if (data.data?.bilibili_cookie) {
          setSavedCookie(data.data.bilibili_cookie);
          setCookie(data.data.bilibili_cookie);
        }
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    if (!cookie.trim()) return;

    setSaving(true);
    try {
      await setConfigValue("bilibili.cookie", cookie.trim());
      setSavedCookie(cookie.trim());
    } catch (error) {
      console.error("Failed to save cookie:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleClear = async () => {
    setSaving(true);
    try {
      await setConfigValue("bilibili.cookie", "");
      setCookie("");
      setSavedCookie("");
    } catch (error) {
      console.error("Failed to clear cookie:", error);
    } finally {
      setSaving(false);
    }
  };

  const isLoggedIn = savedCookie.includes("SESSDATA");

  return (
    <div className="max-w-3xl mx-auto p-6">
      <h1 className="text-2xl font-bold mb-6">Bilibili</h1>

      {/* Status */}
      <div className="mb-6 flex items-center gap-2">
        <span
          className={`inline-block w-2 h-2 rounded-full ${
            isLoggedIn ? "bg-green-500" : "bg-zinc-400"
          }`}
        />
        <span className="text-sm text-zinc-600 dark:text-zinc-400">
          {isLoggedIn ? "已登录" : "未登录"}
        </span>
      </div>

      {/* Cookie Input */}
      <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-4">
        <label className="block text-sm font-medium mb-2">
          Bilibili Cookie
        </label>
        <p className="text-xs text-zinc-500 dark:text-zinc-400 mb-3">
          粘贴 Cookie 以下载会员或登录内容
        </p>
        <textarea
          value={cookie}
          onChange={(e) => setCookie(e.target.value)}
          placeholder="SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx"
          className="w-full h-24 px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none font-mono"
          disabled={!isConnected || saving}
        />

        {/* Buttons */}
        <div className="flex gap-2 mt-3">
          <button
            onClick={handleSave}
            disabled={!isConnected || saving || !cookie.trim()}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? "..." : "保存"}
          </button>
          {isLoggedIn && (
            <button
              onClick={handleClear}
              disabled={!isConnected || saving}
              className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 border border-red-300 dark:border-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              清除
            </button>
          )}
        </div>
      </div>

      {/* Instructions */}
      <div className="mt-6">
        <button
          onClick={() => setShowInstructions(!showInstructions)}
          className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
        >
          <span>{showInstructions ? "▼" : "▶"}</span>
          获取 Cookie 的方法
        </button>

        {showInstructions && (
          <div className="mt-3 p-4 bg-zinc-50 dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-700">
            <ol className="space-y-2 text-sm text-zinc-600 dark:text-zinc-400">
              <li>1. 在浏览器中打开 bilibili.com 并登录</li>
              <li>2. 按 F12 打开开发者工具</li>
              <li>3. 切换到「应用」(Application) 标签</li>
              <li>4. 在左侧找到 Cookies → bilibili.com</li>
              <li>5. 复制以下字段的值：SESSDATA、bili_jct、DedeUserID</li>
              <li>6. 按格式粘贴到上方输入框</li>
            </ol>
            <div className="mt-4 p-3 bg-zinc-100 dark:bg-zinc-800 rounded font-mono text-xs text-zinc-700 dark:text-zinc-300">
              SESSDATA=xxx; bili_jct=xxx; DedeUserID=xxx
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
