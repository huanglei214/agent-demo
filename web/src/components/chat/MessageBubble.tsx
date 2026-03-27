type MessageBubbleProps = {
  role: "user" | "assistant";
  roleLabel: string;
  avatarLabel: string;
  content: string;
  onDebug?: () => void;
};

export function MessageBubble({
  role,
  roleLabel,
  avatarLabel,
  content,
  onDebug,
}: MessageBubbleProps) {
  const paragraphs = splitIntoParagraphs(content);

  return (
    <article className={role === "user" ? "chat-row chat-row-user" : "chat-row chat-row-assistant"}>
      {role === "assistant" ? (
        <div className="chat-avatar border border-white/6 bg-zinc-800/90">{avatarLabel}</div>
      ) : null}
      <div className="chat-bubble-wrap group/bubble">
        <div className="flex items-center gap-2 mb-1">
          <span className="chat-bubble-role m-0">{roleLabel}</span>
          {role === "assistant" && onDebug && (
            <button 
              onClick={onDebug}
              className="opacity-0 group-hover/bubble:opacity-100 transition-opacity text-white/40 hover:text-white/80 p-1.5 rounded bg-white/5 hover:bg-white/10"
              title="Debug Run"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M10 20H14M12 16V20M12 16C10.8954 16 10 15.1046 10 14V11M12 16C13.1046 16 14 15.1046 14 14V11M10 11V7C10 5.89543 10.8954 5 12 5C13.1046 5 14 5.89543 14 7V11M10 11H14" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </button>
          )}
        </div>
        <div
          className={
            role === "user"
              ? "chat-bubble chat-bubble-user border border-white/6 shadow-[0_12px_32px_rgba(0,0,0,0.12)]"
              : "chat-bubble chat-bubble-assistant bg-white/[0.02] shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]"
          }
        >
          <div className={role === "assistant" ? "space-y-3 text-[1.02rem] leading-8 text-zinc-100" : "space-y-2 leading-7"}>
            {paragraphs.map((paragraph, index) => (
              <p key={index}>{paragraph}</p>
            ))}
          </div>
        </div>
      </div>
    </article>
  );
}

function splitIntoParagraphs(content: string) {
  const normalized = content.trim();
  if (!normalized) {
    return [""];
  }

  return normalized
    .split(/\n\s*\n/)
    .map((part) => part.trim())
    .filter(Boolean);
}
