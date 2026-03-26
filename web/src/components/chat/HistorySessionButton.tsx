type HistorySessionButtonProps = {
  active: boolean;
  title: string;
  meta: string;
  detail: string;
  onClick: () => void;
};

export function HistorySessionButton({
  active,
  title,
  meta,
  detail,
  onClick,
}: HistorySessionButtonProps) {
  return (
    <button
      className={
        active
          ? "chat-history-item chat-history-item-active border border-white/10 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
          : "chat-history-item border border-transparent transition hover:border-white/8 hover:bg-white/6"
      }
      type="button"
      onClick={onClick}
    >
      <strong>{title}</strong>
      <span>{meta}</span>
      <small>{detail}</small>
    </button>
  );
}
