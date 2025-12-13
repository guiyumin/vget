import { useApp } from "../context/AppContext";
import { Kuaidi100 } from "../components/Kuaidi100";

export function Kuaidi100Page() {
  const { isConnected } = useApp();

  return (
    <div className="max-w-3xl mx-auto">
      <Kuaidi100 isConnected={isConnected} />
    </div>
  );
}
