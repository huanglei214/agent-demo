import type { ReactNode } from "react";

type SuggestionChipProps = {
  children: ReactNode;
  onClick: () => void;
};

export function SuggestionChip({ children, onClick }: SuggestionChipProps) {
  return (
    <button
      className="chat-suggestion-chip transition hover:-translate-y-0.5 hover:bg-white/8"
      type="button"
      onClick={onClick}
    >
      {children}
    </button>
  );
}
