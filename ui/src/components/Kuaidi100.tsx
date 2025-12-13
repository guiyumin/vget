import { useState } from "react";

interface TrackingRecord {
  time: string;
  ftime: string;
  context: string;
  status: string;
}

interface TrackingResult {
  message: string;
  state: string;
  status: string;
  condition: string;
  ischeck: string;
  com: string;
  nu: string;
  data: TrackingRecord[];
}

interface Kuaidi100Props {
  isConnected: boolean;
}

async function queryKuaidi100(
  trackingNumber: string,
  courier: string
): Promise<{ code: number; data: TrackingResult; message: string }> {
  const res = await fetch("/kuaidi100", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tracking_number: trackingNumber, courier }),
  });
  return res.json();
}

export function Kuaidi100({ isConnected }: Kuaidi100Props) {
  const [trackingNumber, setTrackingNumber] = useState("");
  const [trackingCourier, setTrackingCourier] = useState("auto");
  const [trackingResult, setTrackingResult] = useState<TrackingResult | null>(
    null
  );
  const [trackingError, setTrackingError] = useState("");
  const [isTracking, setIsTracking] = useState(false);
  const [showTracking, setShowTracking] = useState(false);

  const handleTrack = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!trackingNumber.trim() || isTracking) return;

    setIsTracking(true);
    setTrackingError("");
    setTrackingResult(null);

    try {
      const res = await queryKuaidi100(trackingNumber.trim(), trackingCourier);
      if (res.code === 200) {
        setTrackingResult(res.data);
      } else {
        setTrackingError(res.message || "查询失败");
      }
    } catch {
      setTrackingError("网络错误");
    } finally {
      setIsTracking(false);
    }
  };

  const getStateStyle = (state: string) => {
    switch (state) {
      case "3": return "bg-green-100 dark:bg-green-900/50 text-green-600 dark:text-green-500";
      case "0":
      case "5": return "bg-blue-100 dark:bg-blue-900/50 text-blue-600 dark:text-blue-400";
      case "1": return "bg-zinc-300 dark:bg-zinc-700 text-zinc-500";
      case "2":
      case "4":
      case "6": return "bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400";
      default: return "bg-zinc-300 dark:bg-zinc-700 text-zinc-500";
    }
  };

  return (
    <section className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg mb-6 overflow-hidden">
      <div
        className="flex justify-between items-center px-4 py-3 cursor-pointer transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-950"
        onClick={() => setShowTracking(!showTracking)}
      >
        <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200">📦 快递查询 (快递100 API)</h2>
        <span className="text-zinc-500 dark:text-zinc-600 text-xs">{showTracking ? "▼" : "▶"}</span>
      </div>
      {showTracking && (
        <div className="p-4 border-t border-zinc-300 dark:border-zinc-700">
          <form className="flex gap-3 flex-wrap" onSubmit={handleTrack}>
            <input
              type="text"
              value={trackingNumber}
              onChange={(e) => setTrackingNumber(e.target.value)}
              placeholder="输入快递单号..."
              disabled={!isConnected || isTracking}
              className="flex-1 min-w-[200px] px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-md bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            />
            <select
              value={trackingCourier}
              onChange={(e) => setTrackingCourier(e.target.value)}
              disabled={!isConnected || isTracking}
              className="px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-md bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-sm cursor-pointer focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <option value="auto">自动识别</option>
              <option value="shunfeng">顺丰速运</option>
              <option value="yuantong">圆通速递</option>
              <option value="zhongtong">中通快递</option>
              <option value="yunda">韵达快递</option>
              <option value="shentong">申通快递</option>
              <option value="jtexpress">极兔速递</option>
              <option value="jd">京东物流</option>
              <option value="ems">EMS</option>
              <option value="youzhengguonei">邮政快递</option>
              <option value="debangwuliu">德邦物流</option>
              <option value="huitongkuaidi">百世快递</option>
            </select>
            <button
              type="submit"
              disabled={!isConnected || !trackingNumber.trim() || isTracking}
              className="px-4 py-2 border-none rounded-md bg-blue-500 text-white text-sm font-medium cursor-pointer whitespace-nowrap hover:bg-blue-600 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 disabled:cursor-not-allowed transition-colors"
            >
              {isTracking ? "查询中..." : "查询"}
            </button>
          </form>

          {trackingError && (
            <div className="mt-3 px-3 py-2 bg-red-100 dark:bg-red-900/30 rounded-md text-sm text-red-700 dark:text-red-300">
              {trackingError}
            </div>
          )}

          {trackingResult && (
            <div className="mt-4">
              <div className="flex items-center gap-3 p-3 bg-zinc-100 dark:bg-zinc-950 rounded-md mb-3">
                <span className="font-mono text-sm text-zinc-700 dark:text-zinc-200">{trackingResult.nu}</span>
                <span className={`text-xs font-medium px-2 py-1 rounded uppercase ${getStateStyle(trackingResult.state)}`}>
                  {trackingResult.state === "3"
                    ? "✓ 已签收"
                    : trackingResult.state === "0"
                    ? "运输中"
                    : trackingResult.state === "1"
                    ? "已揽收"
                    : trackingResult.state === "2"
                    ? "疑难件"
                    : trackingResult.state === "4"
                    ? "已退签"
                    : trackingResult.state === "5"
                    ? "派送中"
                    : trackingResult.state === "6"
                    ? "退回中"
                    : "未知"}
                </span>
              </div>
              <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto">
                {trackingResult.data?.map((record, idx) => (
                  <div key={idx} className="px-3 py-2 bg-zinc-100 dark:bg-zinc-950 rounded border-l-3 border-l-blue-500">
                    <div className="text-xs text-zinc-500 dark:text-zinc-600 mb-1">
                      {record.ftime || record.time}
                    </div>
                    <div className="text-sm text-zinc-700 dark:text-zinc-200 leading-relaxed">{record.context}</div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
