import AntTooltip from "antd/es/tooltip";
import type { ReactNode } from "react";

type TooltipProps = {
  content: ReactNode;
  children: ReactNode;
};

export function Tooltip({ content, children }: TooltipProps) {
  return (
    <AntTooltip arrow classNames={{ root: "ui-tooltip-content" }} mouseEnterDelay={0} placement="top" title={content}>
      {children}
    </AntTooltip>
  );
}
