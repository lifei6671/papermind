type StatusTone = "success" | "warning" | "danger" | "info";

type StatusBadgeProps = {
  tone: StatusTone;
  children: React.ReactNode;
  className?: string;
};

export function StatusBadge({ tone, children, className }: StatusBadgeProps) {
  const classes = ["status-badge", `status-badge--${tone}`];

  if (className !== undefined) {
    classes.push(className);
  }

  return <span className={classes.join(" ")}>{children}</span>;
}
