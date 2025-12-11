import "./ConfigRow.css";

interface ConfigRowProps {
  label: string;
  value: string;
  options: string[];
  disabled: boolean;
  onChange: (value: string) => void;
}

export function ConfigRow({
  label,
  value,
  options,
  disabled,
  onChange,
}: ConfigRowProps) {
  return (
    <div className="setting-row">
      <span className="setting-label">{label}</span>
      <select
        className="setting-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      >
        {options.map((opt) => (
          <option key={opt} value={opt}>
            {opt}
          </option>
        ))}
      </select>
    </div>
  );
}
