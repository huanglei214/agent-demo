import type { ReactNode } from "react";

type SidebarCardProps = {
  children: ReactNode;
};

export function SidebarCard({ children }: SidebarCardProps) {
  return <div className="chat-sidebar-section rounded-[18px] bg-white/4 p-3.5">{children}</div>;
}
