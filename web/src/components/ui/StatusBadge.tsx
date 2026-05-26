type StatusTone = "success" | "warning" | "danger" | "info";

type StatusBadgeProps = {
  tone: StatusTone;
  children: React.ReactNode;
};

export function StatusBadge({ tone, children }: StatusBadgeProps) {
  return <span className={`status-badge status-badge--${tone}`}>{children}</span>;
}
