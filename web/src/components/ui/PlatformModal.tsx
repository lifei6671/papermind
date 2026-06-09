import AntModal from "antd/es/modal";
import type { ReactNode } from "react";

type PlatformModalProps = {
  children: ReactNode;
  onClose: () => void;
  open: boolean;
  title: string;
  width?: number;
};

export function PlatformModal({ children, onClose, open, title, width = 480 }: PlatformModalProps) {
  return (
    <AntModal
      centered
      className="platform-ant-modal"
      classNames={{
        body: "platform-ant-modal__body",
        title: "platform-ant-modal__title",
      }}
      destroyOnHidden
      footer={null}
      getContainer={false}
      mask={{ closable: false }}
      onCancel={onClose}
      open={open}
      title={title}
      transitionName=""
      maskTransitionName=""
      width={width}
    >
      {children}
    </AntModal>
  );
}
