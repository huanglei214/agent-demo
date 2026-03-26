import type { ReactNode } from "react";

type ComposerFieldProps = {
  children: ReactNode;
  label: string;
  size?: "default" | "medium" | "small";
};

export function ComposerField({ children, label, size }: ComposerFieldProps) {
  const sizeClass =
    size === "medium"
      ? "chat-inline-field chat-inline-field-medium"
      : size === "small"
        ? "chat-inline-field chat-inline-field-small"
        : "chat-inline-field";

  return (
    <label className={sizeClass}>
      <span>{label}</span>
      {children}
    </label>
  );
}
