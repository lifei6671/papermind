import type { ButtonHTMLAttributes } from "react";

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

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
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

export function Button({ className = "", type = "button", variant = "custom", ...props }: ButtonProps) {
  const buttonClassName = [variantClassNames[variant], className].filter(Boolean).join(" ");

  return <button className={buttonClassName || undefined} type={type} {...props} />;
}
