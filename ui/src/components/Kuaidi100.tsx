import { useState } from "react";
import "./Kuaidi100.css";

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

  return (
    <section className="tracking-section">
      <div
        className="tracking-header"
        onClick={() => setShowTracking(!showTracking)}
      >
        <h2>📦 快递查询 (快递100 API)</h2>
        <span className="tracking-toggle">{showTracking ? "▼" : "▶"}</span>
      </div>
      {showTracking && (
        <div className="tracking-content">
          <form className="tracking-form" onSubmit={handleTrack}>
            <input
              type="text"
              value={trackingNumber}
              onChange={(e) => setTrackingNumber(e.target.value)}
              placeholder="输入快递单号..."
              disabled={!isConnected || isTracking}
              className="tracking-input"
            />
            <select
              value={trackingCourier}
              onChange={(e) => setTrackingCourier(e.target.value)}
              disabled={!isConnected || isTracking}
              className="tracking-select"
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
              className="tracking-btn"
            >
              {isTracking ? "查询中..." : "查询"}
            </button>
          </form>

          {trackingError && (
            <div className="tracking-error">{trackingError}</div>
          )}

          {trackingResult && (
            <div className="tracking-result">
              <div className="tracking-summary">
                <span className="tracking-nu">{trackingResult.nu}</span>
                <span
                  className={`tracking-state tracking-state-${trackingResult.state}`}
                >
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
              <div className="tracking-timeline">
                {trackingResult.data?.map((record, idx) => (
                  <div key={idx} className="tracking-record">
                    <div className="tracking-time">
                      {record.ftime || record.time}
                    </div>
                    <div className="tracking-context">{record.context}</div>
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
