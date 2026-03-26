import type { ButtonHTMLAttributes, ReactNode } from "react";

type DrawerActionButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
};

export function DrawerActionButton({
  children,
  ...props
}: DrawerActionButtonProps) {
  return (
    <button
      className="secondary-button border border-white/8 bg-white/6 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
      {...props}
    >
      {children}
    </button>
  );
}
