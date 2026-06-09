import AntButton from "antd/es/button";
import type { ButtonHTMLAttributes, ComponentProps } from "react";

type ButtonVariant =
  | "primary"
  | "secondary"
  | "toolbarPrimary"
  | "toolbarSecondary"
  | "icon"
  | "action"
  | "actionEdit"
  | "actionReset"
  | "actionClose"
  | "actionOpen"
  | "custom";

type AntButtonProps = ComponentProps<typeof AntButton>;

type NativeButtonProps = ButtonHTMLAttributes<HTMLButtonElement>;

type ButtonProps = NativeButtonProps & {
  variant?: ButtonVariant;
};

const variantClassNames: Record<ButtonVariant, string> = {
  primary: "primary-button",
  secondary: "secondary-button",
  toolbarPrimary: "primary-button tenant-create-button",
  toolbarSecondary: "secondary-button tenant-create-button",
  icon: "tenant-icon-button",
  action: "tenant-action-button",
  actionEdit: "tenant-action-button tenant-action--edit",
  actionReset: "tenant-action-button tenant-action--reset",
  actionClose: "tenant-action-button tenant-action--close",
  actionOpen: "tenant-action-button tenant-action--open",
  custom: "",
};

export function Button({ children, className = "", type = "button", variant = "custom", ...props }: ButtonProps) {
  const buttonClassName = [variantClassNames[variant], className].filter(Boolean).join(" ");

  if (variant === "custom") {
    return <button className={buttonClassName || undefined} type={type} {...props}>{children}</button>;
  }

  if (variant === "icon") {
    return (
      <AntButton
        autoInsertSpace={false}
        className={buttonClassName || undefined}
        htmlType={type}
        icon={children}
        {...(props as Omit<AntButtonProps, "autoInsertSpace" | "className" | "htmlType" | "icon" | "type">)}
      />
    );
  }

  return (
    <AntButton
      autoInsertSpace={false}
      className={buttonClassName || undefined}
      htmlType={type}
      {...(props as Omit<AntButtonProps, "autoInsertSpace" | "className" | "htmlType" | "type">)}
    >
      {children}
    </AntButton>
  );
}
