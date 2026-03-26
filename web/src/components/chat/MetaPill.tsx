import type { ReactNode } from "react";

type MetaPillProps = {
  children: ReactNode;
};

export function MetaPill({ children }: MetaPillProps) {
  return (
    <span className="chat-meta-pill border border-white/6 bg-white/6 text-zinc-200 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]">
      {children}
    </span>
  );
}
