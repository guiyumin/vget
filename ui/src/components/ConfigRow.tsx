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
    <div className="ConfigRow flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3">
      <span className="sm:min-w-25 text-sm text-zinc-700 dark:text-zinc-200">
        {label}
      </span>
      <select
        className="flex-1 px-2 py-1.5 border border-zinc-300 dark:border-zinc-700 rounded bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white text-sm cursor-pointer focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
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
