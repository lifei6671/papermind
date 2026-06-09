import AntDrawer from "antd/es/drawer";
import type { ReactNode } from "react";

type PlatformDrawerProps = {
  afterOpenChange?: (open: boolean) => void;
  ariaLabel: string;
  children: ReactNode;
  fullscreen?: boolean;
  nested?: boolean;
  onClose: () => void;
  open: boolean;
};

export function PlatformDrawer({
  afterOpenChange,
  ariaLabel,
  children,
  fullscreen = false,
  nested = false,
  onClose,
  open,
}: PlatformDrawerProps) {
  const drawerClassName = [
    "tenant-resource-drawer",
    fullscreen ? "tenant-resource-drawer--fullscreen" : "tenant-resource-drawer--half",
    open ? "tenant-resource-drawer--open" : "",
    nested ? "tenant-resource-drawer--nested" : "",
  ].filter(Boolean).join(" ");

  return (
    <AntDrawer
      aria-label={ariaLabel}
      className={drawerClassName}
      classNames={{
        body: "tenant-resource-drawer__ant-body",
        wrapper: "tenant-resource-drawer__wrapper",
      }}
      closable={false}
      getContainer={false}
      maskClosable
      afterOpenChange={afterOpenChange}
      onClose={onClose}
      open={open}
      placement="right"
      rootClassName="platform-ant-drawer-root"
      size={fullscreen ? "100vw" : "50vw"}
    >
      {children}
    </AntDrawer>
  );
}
