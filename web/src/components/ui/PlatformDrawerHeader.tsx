import { ArrowLeft } from "lucide-react";
import type { ReactNode } from "react";

type PlatformDrawerHeaderProps = {
  actions?: ReactNode;
  backAriaLabel?: string;
  backLabel?: string;
  onBack: () => void;
  title: string;
};

export function PlatformDrawerHeader({
  actions,
  backAriaLabel,
  backLabel = "返回",
  onBack,
  title,
}: PlatformDrawerHeaderProps) {
  return (
    <header
      aria-label="抽屉头部"
      className="tenant-resource-drawer__head tenant-resource-drawer__head--unified"
    >
      <div className="tenant-resource-drawer__return-line">
        <button
          aria-label={backAriaLabel ?? backLabel}
          className="tenant-resource-drawer__back"
          onClick={onBack}
          type="button"
        >
          <ArrowLeft aria-hidden="true" size={19} />
          <span>{backLabel}</span>
        </button>
        <span aria-hidden="true" className="tenant-resource-drawer__separator">
          |
        </span>
        <span className="tenant-resource-drawer__space-name">{title}</span>
      </div>
      {actions && <div className="tenant-resource-drawer__tools">{actions}</div>}
    </header>
  );
}
