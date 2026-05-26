import type { LucideIcon } from "lucide-react";

type EmptyStateProps = {
  icon?: LucideIcon;
  title: string;
  className?: string;
};

export function EmptyState({ icon: Icon, title, className = "" }: EmptyStateProps) {
  return (
    <div className={`empty-state ${className}`.trim()}>
      {Icon && (
        <span className="empty-state__icon">
          <Icon aria-hidden="true" size={28} strokeWidth={1.7} />
        </span>
      )}
      <p>{title}</p>
    </div>
  );
}
