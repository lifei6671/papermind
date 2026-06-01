import { RefreshCw } from "lucide-react";

type RefreshIconProps = {
  active?: boolean;
  size?: number;
};

export function RefreshIcon({ active = false, size = 16 }: RefreshIconProps) {
  return (
    <RefreshCw
      aria-hidden="true"
      className={active ? "tenant-refresh-icon tenant-refresh-icon--spinning" : "tenant-refresh-icon"}
      size={size}
    />
  );
}
