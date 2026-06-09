import Tag from "antd/es/tag";
import type { ReactNode } from "react";

type StatusTone = "success" | "warning" | "danger" | "info";

type StatusBadgeProps = {
  tone: StatusTone;
  children: ReactNode;
  className?: string;
};

const statusBadgeColorByTone: Record<StatusTone, string> = {
  danger: "error",
  info: "processing",
  success: "success",
  warning: "warning",
};

export function StatusBadge({ tone, children, className }: StatusBadgeProps) {
  const classes = ["status-badge", `status-badge--${tone}`];

  if (className !== undefined) {
    classes.push(className);
  }

  return (
    <Tag className={classes.join(" ")} color={statusBadgeColorByTone[tone]}>
      {children}
    </Tag>
  );
}
